package pubsub

import (
	"encoding/json"
	"errors"

	"github.com/nats-io/nats.go"
)

type Command string

const (
	Broadcast      Command = "broadcast"
	PrivateMessage Command = "private_message"
)

type Message struct {
	Command   Command `json:"command,omitempty"`
	From      string  `json:"from,omitempty"`
	To        string  `json:"to,omitempty"`
	Channel   string  `json:"channel,omitempty"`
	Message   string  `json:"message,omitempty"`
	SessionID string  `json:"session_id,omitempty"`
	CreatedAt string  `json:"created_at,omitempty"`
}

func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *Message) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func NewMessage(command Command, from, to, message, sessionID string) Message {
	return Message{
		Command:   command,
		From:      from,
		To:        to,
		Message:   message,
		SessionID: sessionID,
	}
}

type Broker struct {
	conn *nats.Conn
}

func NewBroker(conn *nats.Conn) Broker {
	return Broker{conn}
}

func (b *Broker) SubscribeAsync(subject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	if subject == "" {
		return nil, errors.New("subject is empty")
	}

	return b.conn.Subscribe(subject, handler)
}

func (b *Broker) SubscribeChannel(subject string) (*nats.Subscription, chan *nats.Msg, error) {
	if subject == "" {
		return nil, nil, errors.New("subject is empty")
	}

	ch := make(chan *nats.Msg)
	sub, err := b.conn.ChanSubscribe(subject, ch)
	if err != nil {
		return nil, nil, err
	}

	return sub, ch, nil
}

/*
{
	"command": "broadcast",
	"from": "rislah",
	"to": "",
	"channel": "1",
	"sessionID": "md5",
	"createdAt": "1699183906"
}

{
	"command": "private_message",
	"from": "rislah",
	"to": "alt1234",
	"channel": "5",
	"session_id": "md5"
	"created_at": "162631962"
}

{
	"type": "private_message",
	"payload": {
		"username": "rislah",
		"channel": "5",
		"message": "hey"
	},
}
*/
