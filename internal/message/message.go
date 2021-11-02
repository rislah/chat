package message

import "encoding/json"

type MessageType string

const (
	JoinChannel        MessageType = "join_channel"
	JoinedChannel      MessageType = "joined_channel"
	ChannelMessage     MessageType = "channel_message"
	UnknownMessageType MessageType = "unknown_command"
	AlreadyInChannel   MessageType = "already_in_channel"
	NotInChannel       MessageType = "not_in_channel"
	GuestNotAllowed    MessageType = "guest_not_allowed"
)

func (m MessageType) Message() Message {
	return Message{Type: m}
}

type Message struct {
	Type    MessageType    `json:"type"`
	Payload MessagePayload `json:"payload"`
}

func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *Message) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

type MessagePayload struct {
	Username string `json:"username,omitempty"`
	Channel  string `json:"channel,omitempty"`
	IsGuest  bool   `json:"is_guest,omitempty"`
	Message  string `json:"message,omitempty"`
}
