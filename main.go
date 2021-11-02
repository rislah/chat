package main

import (
	"chat/internal/server"
	"flag"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type asf struct {
}

func init() {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logger, _ := config.Build()
	_ = zap.ReplaceGlobals(logger)
}

var addr = flag.String("addr", ":8080", "")

func main() {
	flag.Parse()
	nc, err := nats.Connect(nats.DefaultURL, func(options *nats.Options) error {
		options.Timeout = 5 * time.Second
		return nil
	})

	if err != nil {
		zap.L().Fatal("nats connect", zap.Error(err))
	}

	serverConfig := &server.Config{
		NatsConn: nc,
		Addr:     *addr,
		Path:     "/",
	}

	srv := server.NewServer(serverConfig)
	srv.Start()
}
