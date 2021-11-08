package main

import (
	"chat/internal/auth"
	"chat/internal/server"
	"flag"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	log "github.com/sirupsen/logrus"
)

var (
	addr        = flag.String("addr", ":8080", "")
	environment = "devel"
)

func init() {
	// config := zap.NewDevelopmentConfig()
	// config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	// logger, _ := config.Build()
	// _ = zap.ReplaceGlobals(logger)
	switch environment {
	case "devel":
		log.SetFormatter(&log.TextFormatter{})
		log.SetLevel(log.DebugLevel)
	}

	log.SetOutput(os.Stdout)
}

func main() {
	flag.Parse()
	nc, err := nats.Connect(nats.DefaultURL, func(options *nats.Options) error {
		options.Timeout = 5 * time.Second
		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	jwt := auth.NewHS256Wrapper("test")
	serverConfig := &server.Config{
		NatsConn: nc,
		Addr:     *addr,
		Path:     "/",
		Jwt:      jwt,
	}

	token, _ := jwt.Encode(auth.NewUserClaims("kasutaja123", "registered"))
	log.Debug(token)

	srv := server.NewServer(serverConfig)
	srv.Start()
}
