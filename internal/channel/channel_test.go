package channel_test

import (
	"testing"

	"chat/internal/channel"
	"chat/internal/message"
	"chat/internal/pubsub"
	"chat/internal/testutil"

	natsserver "github.com/nats-io/nats-server/v2/server"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sender struct {
	got message.Message
}

func (s *sender) send(msg message.Message) {
	s.got = msg
}

func TestChannelBroadcast(t *testing.T) {
	test := struct {
		channelName string
		username    string
		sessionID   int
		message     pubsub.Message
		expected    message.Message
	}{
		channelName: "asdf",
		username:    "asd",
		sessionID:   0,
		message:     pubsub.Message{Command: pubsub.Broadcast},
		expected:    message.Message{Type: message.ChannelMessage},
	}

	c := channel.NewChannel(test.channelName)
	s := &sender{}
	c.AddMember(test.username, test.sessionID, s.send)
	assert.Equal(t, 1, c.CountMembers())

	b, err := test.message.Marshal()
	assert.NoError(t, err)

	msg := &nats.Msg{
		Data: b,
	}

	c.Broadcast(msg)
	assert.Equal(t, test.expected, s.got)
}

func TestChannelMemberManagement(t *testing.T) {
	type testSuite struct {
		username      string
		sessionID     int
		delete        bool
		add           bool
		expectedCount int
	}

	testAddDel := []testSuite{
		{"test_user", 0, false, true, 1},
		{"test_user", 0, true, false, 0},
	}

	testDelUnknownUser := []testSuite{
		{"test_user", 0, false, true, 1},
		{"asd", 0, true, false, 1},
	}

	testAddDelMany := []testSuite{
		{"test_user1", 0, false, true, 1},
		{"test_user2", 0, false, true, 2},
		{"test_user3", 0, false, true, 3},
		{"test_user4", 0, false, true, 4},
		{"test_user5", 0, false, true, 5},
		{"test_user1", 0, true, false, 4},
		{"test_user2", 0, true, false, 3},
		{"test_user3", 0, true, false, 2},
		{"test_user4", 0, true, false, 1},
		{"test_user5", 0, true, false, 0},
	}

	tests := []struct {
		scenario  string
		testSuite []testSuite
	}{
		{"testAddDel", testAddDel},
		{"testDelUnknownUser", testDelUnknownUser},
		{"testAddDelMany", testAddDelMany},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			c := channel.NewChannel("")

			for _, ts := range test.testSuite {
				if ts.add {
					c.AddMember(ts.username, ts.sessionID, nil)
					assert.Equal(t, ts.expectedCount, c.CountMembers())
				}

				if ts.delete {
					c.DelMember(ts.username, ts.sessionID)
					assert.Equal(t, ts.expectedCount, c.CountMembers())
				}
			}
		})

	}
}

type channelManagerTestCase struct {
	natsConn       *nats.Conn
	natsServer     *natsserver.Server
	channelManager *channel.Manager
}

func TestChannelManager(t *testing.T) {
	tests := []struct {
		scenario string
		test     func(t *testing.T, testCase channelManagerTestCase)
	}{
		{"add channel", testChannelManagerAddChannel},
		{"add channel exists", testChannelManagerAddChannelExists},
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

			testCase := channelManagerTestCase{
				natsConn:       nc,
				natsServer:     srv,
				channelManager: channel.NewManager(nc),
			}

			test.test(t, testCase)
		})
	}

}

func testChannelManagerAddChannel(t *testing.T, testCase channelManagerTestCase) {
	channelName := "test_channel"
	expected := channel.NewChannel(channelName)

	testCase.channelManager.Add(expected)

	got := testCase.channelManager.Get(channelName)
	assert.Equal(t, expected, got)
}

func testChannelManagerAddChannelExists(t *testing.T, testCase channelManagerTestCase) {
	channelName := "test_channel"

	expected := channel.NewChannel(channelName)
	testCase.channelManager.Add(expected)

	got := testCase.channelManager.Get(channelName)
	assert.Equal(t, expected, got)

	testCase.channelManager.Add(expected)

	got = testCase.channelManager.Get(channelName)
	assert.Equal(t, expected, got)
}

// func testChannelManagerJoinChannelDoesntExist(t *testing.T, testCase channelManagerTestCase) {
// 	var (
// 		username     = "user"
// 		channelName  = "test_channel1"
// 		sessionID    = 0
// 		numSubsStart = int(testCase.natsServer.NumSubscriptions())
// 	)
// 	channel, err := testCase.channelManager.JoinChannel(username, channelName, sessionID, nil)
// 	assert.NoError(t, err)

// 	testutil.AssertNumSubscriptions(t, testCase.natsServer, numSubsStart+1)

// 	got := testCase.channelManager.Get(channelName)
// 	assert.Equal(t, channel, got)
// 	assert.Equal(t, 1, got.CountMembers())
// 	assert.Equal(t, channelName, got.Name())

// 	testutil.AssertNumSubscriptions(t, testCase.natsServer, numSubsStart+1)
// }

// func testChannelManagerPurgeEmpty(t *testing.T, testCase channelManagerTestCase) {
// 	var (
// 		username     = "user"
// 		channelName  = "test_channel1"
// 		sessionID    = 0
// 		numSubsStart = int(testCase.natsServer.NumSubscriptions())
// 	)
// 	channel, err := testCase.channelManager.JoinChannel(username, channelName, sessionID, nil)
// 	assert.NoError(t, err)

// 	testutil.AssertNumSubscriptions(t, testCase.natsServer, numSubsStart+1)

// 	time.Sleep(10 * time.Millisecond)
// 	numSubs := int(testCase.natsServer.NumSubscriptions())
// 	assert.Equal(t, numSubsStart+1, numSubs)

// 	got := testCase.channelManager.Get(channelName)
// 	assert.Equal(t, channel, got)
// 	assert.Equal(t, 1, got.CountMembers())

// 	got.DelMember(username, sessionID)
// 	assert.Equal(t, 0, got.CountMembers())

// 	testutil.AssertNumSubscriptions(t, testCase.natsServer, numSubsStart+1)

// 	testCase.channelManager.PurgeEmpty()
// 	got = testCase.channelManager.Get(channelName)
// 	assert.Nil(t, got)

// 	testutil.AssertNumSubscriptions(t, testCase.natsServer, numSubsStart)
// }
