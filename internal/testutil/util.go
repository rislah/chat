package testutil

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
)

var DefaultNatsTestOptions = natsserver.Options{
	Host:                  "127.0.0.1",
	Port:                  natsserver.RANDOM_PORT,
	NoLog:                 false,
	NoSigs:                true,
	MaxControlLine:        4096,
	DisableShortFirstPing: true,
}

func StartNatsServer(opts *natsserver.Options) (*natsserver.Server, error) {
	if opts == nil {
		opts = &DefaultNatsTestOptions
	}

	srv, err := natsserver.NewServer(opts)
	if err != nil {
		return nil, err
	}

	go srv.Start()

	if !srv.ReadyForConnections(10 * time.Second) {
		panic("unable to start nats server")
	}

	return srv, nil
}

func AssertNumSubscriptions(t *testing.T, server *natsserver.Server, expected int) {
	time.Sleep(10 * time.Millisecond)
	numSubs := int(server.NumSubscriptions())
	assert.Equal(t, expected, numSubs)
}

func NatsConnectionURL(srv *natsserver.Server) string {
	return fmt.Sprintf("nats://%s", srv.Addr().String())
}

func GenerateNumberBetween(min, max int) int {
	rand.Seed(time.Now().UnixMicro())
	n := rand.Intn(max-min+1) + min
	return n
}
