package client

import (
	"chat/internal/command"
	"chat/internal/message"
	"chat/internal/pubsub"
	"encoding/json"
	"sync"

	"chat/internal/websocket"

	log "github.com/sirupsen/logrus"

	"github.com/nats-io/nats.go"
)

type connectedChannel struct {
	sessionID          int
	clientSubscription *nats.Subscription
}

type client struct {
	websocketConn  websocket.Connection
	commandHandler *command.Handler
	// map key is channel name
	connectedChannels map[string]connectedChannel

	writerCh  chan message.Message
	messageCh chan message.Message
	pubsubCh  chan pubsub.Message

	user     user
	clientID int

	quitOnce     sync.Once
	quitCh       chan struct{}
	serverQuitCh chan struct{}
}

const (
	sendChannelBuffer = 20
	readChannelBuffer = 1
)

func NewClient(connection websocket.Connection, ch *command.Handler, clientID int, serverQuitCh chan struct{}) (*client, error) {
	usr := user{}
	if err := usr.newFromCtx(connection.Context()); err != nil {
		return nil, err
	}

	return &client{
		connectedChannels: make(map[string]connectedChannel),
		websocketConn:     connection,
		clientID:          clientID,
		commandHandler:    ch,
		pubsubCh:          make(chan pubsub.Message, readChannelBuffer),
		quitCh:            make(chan struct{}),
		messageCh:         make(chan message.Message, readChannelBuffer),
		writerCh:          make(chan message.Message, sendChannelBuffer),
		serverQuitCh:      serverQuitCh,
		user:              usr,
	}, nil
}

func (c *client) close() {
	c.quitOnce.Do(func() { close(c.quitCh) })
	c.websocketConn.Close()
}

func (c *client) leaveChannels() {
	for name, conn := range c.connectedChannels {
		if conn.clientSubscription != nil {
			if err := conn.clientSubscription.Unsubscribe(); err != nil {
				log.WithError(err).Error("unsubscribing channel")
			}
		}

		req := command.NewLeaveChannelReq(name, c.user.username, conn.sessionID)
		c.commandHandler.LeaveChannel(req)
	}
}

func (c *client) Serve() {
	defer c.close()
	defer c.leaveChannels()

	go c.connectionReader()
	go c.pubsubReader()
	go c.commandExecutor()

	for {
		select {
		case msg := <-c.writerCh:
			if err := c.websocketConn.WriteJSON(msg); err != nil {
				log.WithError(err).Error("writing reply")
				return
			}
		case <-c.quitCh:
			return
		}
	}
}

func (c *client) connectionReader() {
	defer c.close()
	defer close(c.messageCh)

	for {
		data, err := c.websocketConn.Read()
		if err != nil {
			return
		}

		if data == nil {
			continue
		}

		msg := message.Message{}
		if err := json.Unmarshal(data, &msg); err != nil {
			return
		}

		select {
		case c.messageCh <- msg:
		case <-c.quitCh:
			return
		case <-c.serverQuitCh:
			return
		}
	}
}

func (c *client) EnqueueMessage(msg message.Message) {
	select {
	case c.writerCh <- msg:
	default:
		// buffer is full
		c.close()
	}
}

func (c *client) pubsubReader() {
	for {
		select {
		case p := <-c.pubsubCh:
			fields := log.Fields{
				"command":   p.Command,
				"channel":   p.Channel,
				"from":      p.From,
				"to":        p.To,
				"message":   p.Message,
				"sessionID": p.SessionID,
				"createdAt": p.CreatedAt,
			}
			log.WithFields(fields).Debug("pubsub message")

			msg := message.FromPubSub(p)
			c.messageCh <- msg
		case <-c.quitCh:
			return
		}
	}
}

func (c *client) commandExecutor() {
	for {
		select {
		case msg := <-c.messageCh:
			fields := log.Fields{
				"type":     msg.Type,
				"channel":  msg.Payload.Channel,
				"from":     msg.Payload.From,
				"message":  msg.Payload.Message,
				"username": msg.Payload.Username,
			}

			log.WithFields(fields).Debug("channel message")

			err := c.execute(msg)
			if err != nil {
				log.WithFields(fields).WithError(err).Warn("executing command executor")
				return
			}
		case <-c.quitCh:
			return
		case <-c.serverQuitCh:
			return
		}
	}
}

func (c *client) execute(msg message.Message) error {
	switch msg.Type {
	case message.JoinChannel:
		return c.joinChannel(msg)
	case message.ChannelMessage:
		return c.channelMessage(msg)
	case message.PrivateMessage:
		return c.privateMessage(msg)
	default:
		reply := message.UnknownMessageType.Message()
		c.EnqueueMessage(reply)
	}
	return nil
}

func (c *client) privateMessage(msg message.Message) error {
	_, inChannel := c.connectedChannels[msg.Payload.Channel]
	if !inChannel {
		reply := message.NotInChannel.Message()
		c.EnqueueMessage(reply)
		return nil
	}

	if c.user.guest {
		reply := message.GuestNotAllowed.Message()
		c.EnqueueMessage(reply)
		return nil
	}

	req := command.NewPrivateMessageReq(c.user.username, msg.Payload.Username, msg.Payload.Channel, msg.Payload.Message)
	c.commandHandler.PrivateMessage(req)

	return nil
}

func (c *client) joinChannel(msg message.Message) error {
	_, inChannel := c.connectedChannels[msg.Payload.Channel]
	if inChannel {
		reply := message.AlreadyInChannel.Message()
		c.EnqueueMessage(reply)
		return nil
	}

	sessionID := len(c.connectedChannels) + 1

	req := command.NewJoinChannelReq(
		c.user.username,
		msg.Payload.Channel,
		c.user.guest,
		sessionID,
		c.EnqueueMessage,
		c.faninToClient,
	)

	sub, err := c.commandHandler.JoinChannel(req)
	if err != nil {
		return err
	}

	c.connectedChannels[msg.Payload.Channel] = connectedChannel{
		sessionID:          sessionID,
		clientSubscription: sub,
	}

	reply := message.Message{Type: message.JoinedChannel,
		Payload: message.Payload{
			Username: c.user.username,
			Channel:  msg.Payload.Channel,
		},
	}

	c.EnqueueMessage(reply)

	return nil
}

func (c *client) channelMessage(msg message.Message) error {
	if c.user.guest {
		reply := message.GuestNotAllowed.Message()
		c.EnqueueMessage(reply)
		return nil
	}

	_, inChannel := c.connectedChannels[msg.Payload.Channel]
	if !inChannel {
		reply := message.NotInChannel.Message()
		c.EnqueueMessage(reply)
		return nil
	}

	req := command.NewChannelMessageReq(msg.Payload.Channel, c.user.username, "", msg.Payload.Message)
	err := c.commandHandler.ChannelMessage(req)
	if err != nil {
		return err
	}

	return nil
}

func (c *client) faninToClient(nm *nats.Msg) {
	msg := pubsub.Message{}
	if err := msg.Unmarshal(nm.Data); err != nil {
		log.WithError(err).Error("unmarshalling pubsub message")
		return
	}
	c.pubsubCh <- msg
}
