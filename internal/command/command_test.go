package command_test

import (
	"chat/internal/channel"
	"chat/internal/command"
	"chat/internal/message"
	"chat/internal/pubsub"
	"chat/internal/testutil"
	"strconv"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSender struct{ got message.Message }

func (fs *fakeSender) Send(msg message.Message) {
	fs.got = msg
}

type commandTestCase struct {
	cmd            *command.Handler
	channelManager *channel.Manager
	natsConn       *nats.Conn
	natsServer     *natsserver.Server
}

func TestCommand(t *testing.T) {
	tests := []struct {
		scenario string
		test     func(t *testing.T, testCase commandTestCase)
	}{
		{"join channel guest", testJoinChannelGuest},
		{"join channel user", testJoinChannelUser},
		{"channel message", testChannelMessage},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			srv, err := testutil.StartNatsServer(nil)
			require.NoError(t, err)
			defer srv.Shutdown()

			connURL := testutil.NatsConnectionURL(srv)
			nc, err := nats.Connect(connURL)
			require.NoError(t, err)

			defer nc.Close()

			channelManager := channel.NewManager(nc)
			broker := pubsub.NewBroker(nc)
			testCase := commandTestCase{
				natsConn:       nc,
				natsServer:     srv,
				channelManager: channelManager,
				cmd:            command.NewHandler(nc, channelManager, &broker),
			}

			test.test(t, testCase)
		})

	}
}

func testJoinChannelUser(t *testing.T, testCase commandTestCase) {
	var (
		msgHandler   = func(msg *nats.Msg) {}
		username     = "user"
		sessionID    = 0
		channelname  = "channel"
		guest        = false
		sender       = &fakeSender{}
		numSubsStart = int(testCase.natsServer.NumSubscriptions())
		req          = command.NewJoinChannelReq(username, channelname, guest, sessionID, sender.Send, msgHandler)
	)

	sub, err := testCase.cmd.JoinChannel(req)
	assert.NoError(t, err)
	assert.NotNil(t, sub)
	testutil.AssertNumSubscriptions(t, testCase.natsServer, numSubsStart+2)

	got := testCase.channelManager.Get(channelname)
	assert.Equal(t, 1, got.CountMembers())
	assert.Equal(t, channelname, got.Name())
}

func testJoinChannelGuest(t *testing.T, testCase commandTestCase) {
	var (
		msgHandler   = func(msg *nats.Msg) {}
		username     = "guest1234"
		sessionID    = 0
		channelname  = "channel"
		guest        = true
		sender       = &fakeSender{}
		numSubsStart = int(testCase.natsServer.NumSubscriptions())
		req          = command.NewJoinChannelReq(username, channelname, guest, sessionID, sender.Send, msgHandler)
	)

	sub, err := testCase.cmd.JoinChannel(req)
	assert.NoError(t, err)
	assert.Nil(t, sub)
	testutil.AssertNumSubscriptions(t, testCase.natsServer, numSubsStart+1)

	got := testCase.channelManager.Get(channelname)
	assert.Equal(t, 1, got.CountMembers())
	assert.Equal(t, channelname, got.Name())
}

func testChannelMessage(t *testing.T, testCase commandTestCase) {
	var (
		msgHandler   = func(msg *nats.Msg) {}
		username     = "test_user"
		sessionID    = 0
		channelname  = "channel"
		guest        = false
		sender       = &fakeSender{}
		msg          = "foo"
		numSubsStart = int(testCase.natsServer.NumSubscriptions())
		req          = command.NewJoinChannelReq(username, channelname, guest, sessionID, sender.Send, msgHandler)
	)

	sub, err := testCase.cmd.JoinChannel(req)
	assert.NoError(t, err)
	assert.NotNil(t, sub)
	testutil.AssertNumSubscriptions(t, testCase.natsServer, numSubsStart+2)

	got := testCase.channelManager.Get(channelname)
	assert.Equal(t, 1, got.CountMembers())
	assert.Equal(t, channelname, got.Name())

	subj := got.Subscription().Subject
	ch := make(chan *nats.Msg)
	_, err = testCase.natsConn.ChanSubscribe(subj, ch)
	assert.NoError(t, err)

	channelMessageReq := command.NewChannelMessageReq(channelname, username, strconv.Itoa(sessionID), msg)
	err = testCase.cmd.ChannelMessage(channelMessageReq)
	assert.NoError(t, err)

	timeout := time.NewTimer(500 * time.Millisecond)

	select {
	case chMsg := <-ch:
		expected := pubsub.Message{Command: pubsub.Broadcast, From: username, Message: msg, SessionID: strconv.Itoa(sessionID)}
		pubsubMsg := pubsub.Message{}
		err = pubsubMsg.Unmarshal(chMsg.Data)
		assert.NoError(t, err)

		assert.Equal(t, expected, pubsubMsg)
	case <-timeout.C:
		assert.Fail(t, "timeout")
	}
}
