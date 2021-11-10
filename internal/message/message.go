package message

import (
	"chat/internal/pubsub"
	"encoding/json"

	"go.uber.org/zap/zapcore"
)

type Type string

const (
	JoinChannel        Type = "join_channel"
	JoinedChannel      Type = "joined_channel"
	ChannelMessage     Type = "channel_message"
	PrivateMessage     Type = "private_message"
	UnknownMessageType Type = "unknown_command"
	AlreadyInChannel   Type = "already_in_channel"
	NotInChannel       Type = "not_in_channel"
	GuestNotAllowed    Type = "guest_not_allowed"
	Ping               Type = "ping"
	Pong               Type = "pong"
)

func (m Type) Message() Message {
	return Message{Type: m}
}

type Message struct {
	Type    Type    `json:"type"`
	Payload Payload `json:"payload,omitempty"`
}

func (m Message) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddString("type", string(m.Type))
	encoder.AddString("username", m.Payload.Username)
	encoder.AddString("from", m.Payload.Username)
	encoder.AddString("channel", m.Payload.Channel)
	encoder.AddString("message", m.Payload.Message)
	encoder.AddBool("guest", m.Payload.IsGuest)
	return nil
}

func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *Message) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func FromPubSub(pmsg pubsub.Message) Message {
	msg := Message{}
	switch pmsg.Command {
	case pubsub.Broadcast:
		msg.Type = ChannelMessage
		msg.Payload.From = pmsg.From
		msg.Payload.Channel = pmsg.Channel
		msg.Payload.Message = pmsg.Message
	case pubsub.PrivateMessage:
		msg.Type = PrivateMessage
		msg.Payload.From = pmsg.From
		msg.Payload.Channel = pmsg.Channel
		msg.Payload.Message = pmsg.Message
	}
	return msg
}

type Payload struct {
	Username string `json:"username,omitempty"`
	From     string `json:"from,omitempty"`
	Channel  string `json:"channel,omitempty"`
	IsGuest  bool   `json:"is_guest,omitempty"`
	Message  string `json:"message,omitempty"`
}
