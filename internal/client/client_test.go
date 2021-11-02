package client

import (
	"chat/internal/message"
	channel "chat/internal/rework"
	"chat/internal/testutil"
	"chat/internal/websocket/websocketfakes"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type clientTestCase struct {
	natsServer     *natsserver.Server
	connection     *websocketfakes.FakeConnection
	channelManager *channel.Manager
	client         *client
}

func TestClient(t *testing.T) {
	tests := []struct {
		scenario string
		test     func(t *testing.T, testCase clientTestCase)
	}{
		{"join channel command", testClientJoinCommand},
		{"unknown command", testClientUnknownCommand},
		{"channel message command", testClientChannelMessage},
		{"close", testClientClose},
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

			connection := &websocketfakes.FakeConnection{}
			channelManager := channel.NewChannelsManager(nc)
			client := NewClient(connection, channelManager, nc, 0, make(chan struct{}))
			defer client.close()

			testCase := clientTestCase{
				connection:     connection,
				channelManager: channelManager,
				client:         client,
				natsServer:     srv,
			}

			test.test(t, testCase)
		})
	}
}

func testClientJoinCommand(t *testing.T, testCase clientTestCase) {
	var (
		numSubsStart = int(testCase.natsServer.NumSubscriptions())
		msg          = message.Message{
			Type: message.JoinChannel,
			Payload: message.MessagePayload{
				Username: "test",
				Channel:  "1",
				IsGuest:  false,
			},
		}
	)

	err := testCase.client.execute(msg)
	assert.NoError(t, err)

	reply := <-testCase.client.sendCh
	assert.Equal(t, message.JoinedChannel, reply.Type)
	testutil.AssertNumSubscriptions(t, testCase.natsServer, numSubsStart+2)

	ch := testCase.channelManager.Get(msg.Payload.Channel)
	assert.NotNil(t, ch)
	members := ch.CountMembers()
	assert.Equal(t, 1, members)

	_, ok := testCase.client.channels[msg.Payload.Channel]
	assert.True(t, ok)
}

func testClientUnknownCommand(t *testing.T, testCase clientTestCase) {
	var (
		numSubsStart = int(testCase.natsServer.NumSubscriptions())
		msg          = message.Message{
			Type: "asd",
			Payload: message.MessagePayload{
				Username: "test",
				Channel:  "1",
				IsGuest:  false,
			},
		}
	)

	err := testCase.client.execute(msg)
	assert.NoError(t, err)
	reply := <-testCase.client.sendCh
	assert.Equal(t, message.UnknownMessageType, reply.Type)
	testutil.AssertNumSubscriptions(t, testCase.natsServer, numSubsStart)
}

func testClientClose(t *testing.T, testCase clientTestCase) {
	var (
		numSubsStart = int(testCase.natsServer.NumSubscriptions())
		msg          = message.Message{
			Type: message.JoinChannel,
			Payload: message.MessagePayload{
				Username: "test",
				Channel:  "1",
				IsGuest:  false,
			},
		}
	)

	err := testCase.client.execute(msg)
	assert.NoError(t, err)
	reply := <-testCase.client.sendCh

	assert.Equal(t, message.JoinedChannel, reply.Type)
	testutil.AssertNumSubscriptions(t, testCase.natsServer, numSubsStart+2)

	connectedChannel, ok := testCase.client.channels[msg.Payload.Channel]
	assert.True(t, ok)
	assert.True(t, connectedChannel.IsValid())

	testCase.client.close()

	callCount := testCase.connection.CloseCallCount()
	assert.Equal(t, 1, callCount)
	assert.False(t, connectedChannel.IsValid())

	_, open := <-testCase.client.quitCh
	assert.False(t, open)
}

func testClientChannelMessage(t *testing.T, testCase clientTestCase) {
	var (
		numSubsStart = int(testCase.natsServer.NumSubscriptions())
		joinMessage  = message.Message{
			Type: message.JoinChannel,
			Payload: message.MessagePayload{
				Username: "test",
				Channel:  "1",
				IsGuest:  false,
			},
		}
		channelMessage = message.Message{
			Type: message.ChannelMessage,
			Payload: message.MessagePayload{
				Username: "test",
				Channel:  "1",
				IsGuest:  false,
				Message:  "foobar",
			},
		}
	)

	err := testCase.client.execute(joinMessage)
	assert.NoError(t, err)

	reply := <-testCase.client.sendCh
	assert.Equal(t, message.JoinedChannel, reply.Type)
	testutil.AssertNumSubscriptions(t, testCase.natsServer, numSubsStart+2)

	_, ok := testCase.client.channels[joinMessage.Payload.Channel]
	assert.True(t, ok)

	err = testCase.client.execute(channelMessage)
	reply = <-testCase.client.sendCh
	assert.NoError(t, err)
	assert.Nil(t, reply)

	select {
	case msg := <-testCase.client.sendCh:
		assert.Equal(t, channelMessage, msg)
	case <-time.After(500 * time.Millisecond):
		assert.Fail(t, "Timeout")
	}
}
