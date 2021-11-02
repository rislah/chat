package command

import (
	"chat/internal/channel"
	"chat/internal/pubsub"
	"fmt"

	"github.com/nats-io/nats.go"
)

type Handler struct {
	natsConn       *nats.Conn
	channelManager *channel.Manager
	broker         *pubsub.Broker
}

func NewHandler(natsConn *nats.Conn, channelManager *channel.Manager, broker *pubsub.Broker) *Handler {
	return &Handler{natsConn, channelManager, broker}
}

type UserJoinChannelReq struct {
	username      string
	channel       string
	isGuest       bool
	sessionID     int
	messageSendFn channel.MessageSendFn
	subMsgHandler nats.MsgHandler
}

func NewUserJoinChannelReq(username string, channel string, guest bool, sessionID int, sendFn channel.MessageSendFn, msgHandler nats.MsgHandler) UserJoinChannelReq {
	return UserJoinChannelReq{username, channel, guest, sessionID, sendFn, msgHandler}
}

func (cmd *Handler) JoinChannel(req UserJoinChannelReq) (*nats.Subscription, error) {
	var (
		err error
		sub *nats.Subscription
	)

	if !req.isGuest {
		subj := buildUserSubjectName(req.username, req.channel)
		sub, err = cmd.broker.SubscribeAsync(subj, req.subMsgHandler)
		if err != nil {
			return nil, err
		}
	}

	ch := cmd.channelManager.Get(req.channel)
	if ch == nil {
		subj := buildChannelSubjectName(req.channel)
		ch = channel.NewChannel(req.channel)
		channelSub, err := cmd.broker.SubscribeAsync(subj, ch.Broadcast)
		if err != nil {
			return nil, err
		}

		ch.SetSub(channelSub)
		cmd.channelManager.Add(ch)
	}

	ch.AddMember(req.username, req.sessionID, req.messageSendFn)

	return sub, nil
}

type ChannelMessageReq struct {
	channel   string
	from      string
	sessionID string
	message   string
}

func NewChannelMessageReq(channel, from, sessionID, message string) ChannelMessageReq {
	return ChannelMessageReq{channel, from, sessionID, message}
}

func (c *Handler) ChannelMessage(req ChannelMessageReq) error {
	subject := buildChannelSubjectName(req.channel)
	msg := pubsub.NewMessage(pubsub.Broadcast, req.from, "", req.message, req.sessionID)
	b, err := msg.Marshal()
	if err != nil {
		return err
	}

	err = c.natsConn.Publish(subject, b)
	if err != nil {
		return err
	}

	return nil
}

type LeaveChannelReq struct {
	channel   string
	username  string
	sessionID int
}

func NewLeaveChannelReq(channel, username string, sessionID int) LeaveChannelReq {
	return LeaveChannelReq{channel, username, sessionID}
}

func (c *Handler) LeaveChannel(req LeaveChannelReq) {
	ch := c.channelManager.Get(req.channel)
	if ch == nil {
		return
	}
	ch.DelMember(req.username, req.sessionID)
}

func buildUserSubjectName(username, channel string) string {
	return fmt.Sprintf("channel.%s.%s", channel, username)
}

func buildChannelSubjectName(channel string) string {
	return fmt.Sprintf("channel.%s", channel)
}
