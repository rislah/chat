package server

import (
	"chat/internal/auth"
	"chat/internal/channel"
	"chat/internal/client"
	"chat/internal/pubsub"
	"chat/internal/websocket"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type Config struct {
	NatsConn *nats.Conn
	Jwt      auth.JWTWrapper
	Addr     string
	Path     string
}

type Server struct {
	wsserver       websocket.Server
	broker         pubsub.Broker
	natsConn       *nats.Conn
	channelManager *channel.Manager
	connCh         chan websocket.Connection
	quitCh         chan struct{}
	quitOnce       sync.Once
}

func NewServer(conf *Config) *Server {
	if conf == nil {
		panic("conf must not be nil")
	}

	srv := &Server{
		broker:         pubsub.NewBroker(conf.NatsConn),
		quitCh:         make(chan struct{}),
		channelManager: channel.NewChannelsManager(conf.NatsConn),
		connCh:         make(chan websocket.Connection),
		natsConn:       conf.NatsConn,
	}

	router := mux.NewRouter()
	router.Use(auth.ContextMiddleware(conf.Jwt))
	router.HandleFunc(conf.Path, srv.handler).Methods("GET")
	wsserver := websocket.NewServer(router, conf.Addr)

	srv.wsserver = wsserver

	return srv
}

func (s *Server) Start() {
	defer s.close()

	go s.wsserver.Serve()
	// go s.statsPrinter()
	go s.emptyPurger()

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR1, syscall.SIGUSR2)

	var id int
	for {
		select {
		case conn := <-s.connCh:
			id++
			client, err := client.NewClient(conn, s.channelManager, s.natsConn, &s.broker, id, s.quitCh)
			if err != nil {
				log.WithError(err).Warn("creating new client")
			}

			go client.Serve()
		case <-sigchan:
			return
		}
	}
}

func (s *Server) printStats() {
	memberCount := 0
	channels := s.channelManager.List()

	for _, channel := range channels {
		memberCount += channel.CountMembers()
	}

	log.WithFields(log.Fields{"channels": len(channels), "members": memberCount}).Info("stats")
}

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	conn, err := s.wsserver.Upgrade(w, r, http.Header{})
	if err != nil {
		zap.L().Warn("upgrading connection", zap.Error(err))
		return
	}

	s.connCh <- conn
}

func (s *Server) close() {
	s.quitOnce.Do(func() { close(s.quitCh) })
	s.wsserver.Close()
	channels := s.channelManager.List()
	for _, channel := range channels {
		channel.Close()
	}
}

func (s *Server) statsPrinter() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.printStats()
		case <-s.quitCh:
			return
		}
	}
}

func (s *Server) emptyPurger() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.channelManager.PurgeEmpty()
		case <-s.quitCh:
			return
		}
	}
}
