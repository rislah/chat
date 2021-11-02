package client

import (
	"chat/internal/channel"
	"chat/internal/command"
	"chat/internal/keys"
	"chat/internal/message"
	"chat/internal/pubsub"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"strconv"
	"sync"

	"chat/internal/websocket"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type client struct {
	connection     websocket.Connection
	commandHandler *command.Handler
	channels       map[string]*nats.Subscription

	sendCh   chan message.Message
	readCh   chan message.Message
	pubsubCh chan pubsub.Message

	ctx      context.Context
	id       int
	username string
	guest    bool

	quitOnce     sync.Once
	quitCh       chan struct{}
	serverQuitCh chan struct{}
}

const (
	sendChannelBuffer = 20
	readChannelBuffer = 1
)

func NewClient(connection websocket.Connection, channelManager *channel.Manager, natsConn *nats.Conn, clientID int, serverQuitCh chan struct{}) *client {
	broker := pubsub.NewBroker(natsConn)
	ch := command.NewHandler(natsConn, channelManager, &broker)

	c := &client{
		channels:       make(map[string]*nats.Subscription),
		commandHandler: ch,
		connection:     connection,
		ctx:            connection.Context(),
		id:             clientID,
		pubsubCh:       make(chan pubsub.Message, readChannelBuffer),
		quitCh:         make(chan struct{}),
		readCh:         make(chan message.Message, readChannelBuffer),
		sendCh:         make(chan message.Message, sendChannelBuffer),
		serverQuitCh:   serverQuitCh,
	}

	c.init()

	return c
}

func (c *client) init() {
	guestStr := c.ctx.Value(keys.GuestCtx).(string)
	guest, _ := strconv.ParseBool(guestStr)

	user := c.ctx.Value(keys.UsernameCtx).(string)
	c.guest = guest

	if guest {
		user = c.generateGuestName()
	}

	c.username = user
}

func (c *client) generateGuestName() string {
	id := make([]byte, 2)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		zap.L().Warn("generateName", zap.Error(err))
		return ""
	}
	return fmt.Sprintf("%x%d", id, c.id)
}

func (c *client) Serve() {
	defer c.leaveChannels()
	defer c.close()

	go c.messageReader()
	go c.commandExecutor()

	c.messageWriter()
}

func (c *client) messageReader() {
	defer c.close()
	defer close(c.readCh)

	for {
		msg, err := c.readMessage()
		if err != nil {
			return
		}

		select {
		case c.readCh <- msg:
		case <-c.quitCh:
			return
		case <-c.serverQuitCh:
			return
		}
	}
}

func (c *client) messageWriter() {
	for {
		select {
		case msg := <-c.sendCh:
			if err := c.writeMessage(msg); err != nil {
				zap.L().Warn("writing conn reply", zap.Error(err))
				return
			}
		case <-c.quitCh:
			return
		case <-c.serverQuitCh:
			return
		}
	}
}

func (c *client) EnqueueMessage(msg message.Message) {
	select {
	case c.sendCh <- msg:
	default:
		c.close()
	}
}

func (c *client) commandExecutor() {
	for {
		select {
		case msg := <-c.readCh:
			err := c.execute(msg)
			if err != nil {
				zap.L().Warn("executing command executor", zap.Error(err))
				return
			}
		case p := <-c.pubsubCh:
			msg := c.parseBrokerMessage(p)
			if msg == nil {
				continue
			}

			err := c.execute(*msg)
			if err != nil {
				zap.L().Warn("executing command executor", zap.Error(err))
				return
			}
		case <-c.quitCh:
			return
		case <-c.serverQuitCh:
			return
		}
	}
}

func (c *client) inChannel(name string) bool {
	_, ok := c.channels[name]
	return ok
}

func (c *client) execute(msg message.Message) error {
	switch msg.Type {
	case message.JoinChannel:
		err := c.joinChannel(msg)
		if err != nil {
			return err
		}
	case message.ChannelMessage:
		err := c.channelMessage(msg)
		if err != nil {
			return err
		}
	default:
		reply := message.UnknownMessageType.Message()
		c.EnqueueMessage(reply)
	}
	return nil
}

func (c *client) joinChannel(msg message.Message) error {
	if c.inChannel(msg.Payload.Channel) {
		reply := message.AlreadyInChannel.Message()
		c.EnqueueMessage(reply)
		return nil
	}

	req := command.NewUserJoinChannelReq(
		c.username,
		msg.Payload.Channel,
		c.guest,
		c.id,
		c.EnqueueMessage,
		c.forwardNatsMessageToClient,
	)

	sub, err := c.commandHandler.JoinChannel(req)
	if err != nil {
		return err
	}

	c.channels[msg.Payload.Channel] = sub

	reply := message.Message{Type: message.JoinedChannel,
		Payload: message.MessagePayload{
			Username: c.username,
			Channel:  msg.Payload.Channel,
		},
	}

	c.EnqueueMessage(reply)

	return nil
}

func (c *client) channelMessage(msg message.Message) error {
	if c.guest {
		reply := message.GuestNotAllowed.Message()
		c.EnqueueMessage(reply)
		return nil
	}

	if !c.inChannel(msg.Payload.Channel) {
		reply := message.NotInChannel.Message()
		c.EnqueueMessage(reply)
		return nil
	}

	req := command.NewChannelMessageReq(msg.Payload.Channel, c.username, "", msg.Payload.Message)
	err := c.commandHandler.ChannelMessage(req)
	if err != nil {
		return err
	}

	return nil
}

func (c *client) close() {
	c.quitOnce.Do(func() { close(c.quitCh) })
	_ = c.connection.Close()
}

func (c *client) leaveChannels() {
	for name, channel := range c.channels {
		c.commandHandler.LeaveChannel(command.NewLeaveChannelReq(name, c.username, c.id))
		if channel != nil {
			if err := channel.Unsubscribe(); err != nil {
				zap.L().Warn("unsubscribing channel", zap.Error(err))
			}
		}
	}
}

func (c *client) writeMessage(msg message.Message) error {
	err := c.connection.WriteJSON(&msg)
	if err != nil {
		return err
	}
	return nil
}

func (c *client) readMessage() (message.Message, error) {
	msg := message.Message{}
	if err := c.connection.ReadJSON(&msg); err != nil {
		return msg, err
	}
	return msg, nil
}

func (c *client) forwardNatsMessageToClient(natsMsg *nats.Msg) {
	pubsubMsg := pubsub.Message{}
	pubsubMsg.Unmarshal(natsMsg.Data)
	c.pubsubCh <- pubsubMsg
}

func (c *client) parseBrokerMessage(p pubsub.Message) *message.Message {
	switch p.Command {
	case pubsub.Broadcast:
		msg := message.ChannelMessage.Message()
		msg.Payload.Username = c.username
		msg.Payload.Channel = p.Channel
		msg.Payload.Message = p.Message
		return &msg
	default:
		return nil
	}
}
