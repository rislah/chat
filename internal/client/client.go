package client

import (
	"chat/internal/auth"
	"chat/internal/channel"
	"chat/internal/command"
	"chat/internal/ctxkeys"
	"chat/internal/message"
	"chat/internal/pubsub"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"sync"

	"chat/internal/websocket"

	log "github.com/sirupsen/logrus"

	"github.com/nats-io/nats.go"
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
	role     string

	quitOnce     sync.Once
	quitCh       chan struct{}
	serverQuitCh chan struct{}
}

const (
	sendChannelBuffer = 20
	readChannelBuffer = 1
)

func NewClient(connection websocket.Connection, channelManager *channel.Manager, natsConn *nats.Conn, broker *pubsub.Broker, clientID int, serverQuitCh chan struct{}) (*client, error) {
	c := &client{
		channels:       make(map[string]*nats.Subscription),
		commandHandler: command.NewHandler(natsConn, channelManager, broker),
		connection:     connection,
		ctx:            connection.Context(),
		id:             clientID,
		pubsubCh:       make(chan pubsub.Message, readChannelBuffer),
		quitCh:         make(chan struct{}),
		readCh:         make(chan message.Message, readChannelBuffer),
		sendCh:         make(chan message.Message, sendChannelBuffer),
		serverQuitCh:   serverQuitCh,
	}

	if err := c.init(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *client) init() error {
	val := c.ctx.Value(ctxkeys.User)
	if val == nil {
		name, err := c.generateGuestName()
		if err != nil {
			return err
		}

		c.username = name
		c.guest = true
		return nil
	}

	user, ok := val.(*auth.Context)
	if !ok {
		return errors.New("failed to cast context value to auth context")
	}

	c.username = user.Username
	c.guest = false
	c.role = user.Role

	return nil
}

func (c *client) generateGuestName() (string, error) {
	id := make([]byte, 2)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		return "", err
	}
	return fmt.Sprintf("guest%x%d", id, c.id), nil
}

func (c *client) Serve() {
	defer c.leaveChannels()
	defer c.close()

	go c.connectionReader()
	go c.commandExecutor()

	c.connectionWriter()
}

func (c *client) connectionReader() {
	defer c.close()
	defer close(c.readCh)

	for {
		msg := message.Message{}
		err := c.connection.ReadJSON(&msg)
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

func (c *client) connectionWriter() {
	for {
		select {
		case msg := <-c.sendCh:
			if err := c.connection.WriteJSON(msg); err != nil {
				log.WithError(err).Warn("writing conn reply")
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
			log.WithFields(log.Fields{
				"type":     msg.Type,
				"channel":  msg.Payload.Channel,
				"from":     msg.Payload.From,
				"message":  msg.Payload.Message,
				"username": msg.Payload.Username,
			}).Debug("channel message")

			err := c.execute(msg)
			if err != nil {
				log.WithError(err).Warn("executing command executor")
				return
			}
		case p := <-c.pubsubCh:
			log.WithFields(log.Fields{
				"command":   p.Command,
				"channel":   p.Channel,
				"from":      p.From,
				"to":        p.To,
				"message":   p.Message,
				"sessionID": p.SessionID,
				"createdAt": p.CreatedAt,
			}).Debug("pubsub message")

			msg := message.FromPubSub(p)
			err := c.execute(msg)
			if err != nil {
				log.WithError(err).Warn("executing command executor")
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
		c.faninToClient,
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
				log.WithError(err).Warn("unsubscribe channel")
			}
		}
	}
}

func (c *client) faninToClient(natsMsg *nats.Msg) {
	pubsubMsg := pubsub.Message{}
	pubsubMsg.Unmarshal(natsMsg.Data)
	c.pubsubCh <- pubsubMsg
}
