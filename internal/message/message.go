package message

import (
	"chat/internal/pubsub"
	"encoding/json"

	"go.uber.org/zap/zapcore"
)

type MessageType string

const (
	JoinChannel        MessageType = "join_channel"
	JoinedChannel      MessageType = "joined_channel"
	ChannelMessage     MessageType = "channel_message"
	PrivateMessage     MessageType = "private_message"
	UnknownMessageType MessageType = "unknown_command"
	AlreadyInChannel   MessageType = "already_in_channel"
	NotInChannel       MessageType = "not_in_channel"
	GuestNotAllowed    MessageType = "guest_not_allowed"
	Ping               MessageType = "ping"
	Pong               MessageType = "pong"
)

func (m MessageType) Message() Message {
	return Message{Type: m}
}

type Message struct {
	Type    MessageType    `json:"type"`
	Payload MessagePayload `json:"payload"`
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

type MessagePayload struct {
	Username string `json:"username,omitempty"`
	From     string `json:"from,omitempty"`
	Channel  string `json:"channel,omitempty"`
	IsGuest  bool   `json:"is_guest,omitempty"`
	Message  string `json:"message,omitempty"`
}
