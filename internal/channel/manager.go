package channel

import (
	"sync"

	"github.com/nats-io/nats.go"
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
