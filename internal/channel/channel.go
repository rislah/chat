package channel

import (
	"chat/internal/message"
	"chat/internal/pubsub"
	"sync"

	"github.com/nats-io/nats.go"
	log "github.com/sirupsen/logrus"
)

type MessageSendFn func(msg message.Message)

type Channel struct {
	name    string
	members map[string]map[int]MessageSendFn
	sub     *nats.Subscription
	rw      sync.RWMutex
}

func NewChannel(name string) *Channel {
	return &Channel{
		name:    name,
		members: make(map[string]map[int]MessageSendFn),
	}
}

func (c *Channel) SetSub(sub *nats.Subscription) {
	c.sub = sub
}

func (c *Channel) Broadcast(msg *nats.Msg) {
	pmsg := pubsub.Message{}
	err := pmsg.Unmarshal(msg.Data)
	if err != nil {
		log.WithError(err).Warn("unmarshal pubsub message")
		return
	}

	m := message.FromPubSub(pmsg)
	c.broadcast(m)
}

func (c *Channel) Close() {
	if err := c.sub.Unsubscribe(); err != nil {
		log.WithError(err).Warn("unsubscribe channel")
	}
}

func (c *Channel) broadcast(msg message.Message) {
	log.WithFields(log.Fields{
		"type":     msg.Type,
		"channel":  msg.Payload.Channel,
		"from":     msg.Payload.From,
		"username": msg.Payload.Username,
	}).Debug("broadcasting message")

	members := c.members
	for _, sessions := range members {
		for _, send := range sessions {
			send(msg)
		}
	}
}

func (c *Channel) AddMember(username string, sessionID int, sendFn MessageSendFn) {
	c.rw.RLock()
	sessions, ok := c.members[username]
	if !ok {
		sessions = make(map[int]MessageSendFn)
	}
	c.rw.RUnlock()

	c.rw.Lock()
	defer c.rw.Unlock()

	sessions[sessionID] = sendFn
	c.members[username] = sessions
}

func (c *Channel) DelMember(username string, sessionID int) {
	c.rw.RLock()
	sessions, ok := c.members[username]
	if !ok {
		c.rw.RUnlock()
		return
	}
	c.rw.RUnlock()

	c.rw.Lock()
	defer c.rw.Unlock()

	delete(sessions, sessionID)

	if len(sessions) == 0 {
		delete(c.members, username)
	}
}

func (c *Channel) Name() string {
	return c.name
}

func (c *Channel) CountMembers() int {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return len(c.members)
}

func (c *Channel) Subscription() *nats.Subscription {
	return c.sub
}
