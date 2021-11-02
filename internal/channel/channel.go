package channel

import (
	"chat/internal/message"
	"chat/internal/pubsub"
	"sync"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type Manager struct {
	nc       *nats.Conn
	channels map[string]*Channel
	mu       sync.RWMutex
}

func NewChannelsManager(nc *nats.Conn) *Manager {
	return &Manager{
		nc:       nc,
		channels: make(map[string]*Channel),
	}
}

func (cm *Manager) Add(ch *Channel) {
	cm.mu.RLock()
	channelName := ch.Name()
	if _, ok := cm.channels[channelName]; ok {
		cm.mu.RUnlock()
		// {"purge empty channels", testChannelManagerPurgeEmpty},
		return
	}
	cm.mu.RUnlock()

	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.channels[channelName] = ch
}

func (cm *Manager) Get(channelName string) *Channel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	channel, ok := cm.channels[channelName]
	if !ok {
		return nil
	}

	return channel
}

func (cm *Manager) List() map[string]*Channel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.channels
}

func (cm *Manager) PurgeEmpty() {
	channelsToRemove := []string{}

	cm.mu.RLock()
	for name, channel := range cm.channels {
		count := channel.CountMembers()
		if count == 0 {
			channel.Close()
			channelsToRemove = append(channelsToRemove, name)
		}
	}
	cm.mu.RUnlock()

	if len(channelsToRemove) > 0 {
		cm.mu.Lock()
		for _, channelToRemove := range channelsToRemove {
			delete(cm.channels, channelToRemove)
		}
		cm.mu.Unlock()
	}
}

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
	pm := pubsub.Message{}
	err := pm.Unmarshal(msg.Data)
	if err != nil {
		zap.L().Warn("unmarshalling pubsub message", zap.Error(err))
	}

	switch pm.Command {
	case pubsub.Broadcast:
		m := message.Message{
			Type: message.ChannelMessage,
			Payload: message.MessagePayload{
				Username: pm.From,
				Channel:  pm.Channel,
				Message:  pm.Message,
			},
		}

		c.broadcast(m)
	default:
		return
	}
}

func (c *Channel) Close() {
	if err := c.sub.Unsubscribe(); err != nil {
		zap.L().Warn("unsubscribing channel", zap.Error(err))
	}
}

func (c *Channel) broadcast(message message.Message) {
	members := c.members
	for _, sessions := range members {
		for _, send := range sessions {
			send(message)
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
