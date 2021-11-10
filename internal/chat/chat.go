package chat

import (
	"chat/internal/auth"
	"chat/internal/channel"
	"chat/internal/client"
	"chat/internal/command"
	"chat/internal/pubsub"
	"chat/internal/websocket"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/nats-io/nats.go"
)

type Config struct {
	NatsConn *nats.Conn
	Jwt      auth.JWTWrapper
	Addr     string
	Path     string
}

type Chat struct {
	wsserver       websocket.Server
	broker         pubsub.Broker
	natsConn       *nats.Conn
	channelManager *channel.Manager
	commandHandler *command.Handler
	connCh         chan websocket.Connection
	quitCh         chan struct{}
	quitOnce       sync.Once
}

func New(conf *Config) *Chat {
	if conf == nil {
		panic("conf must not be nil")
	}

	cm := channel.NewManager(conf.NatsConn)
	broker := pubsub.NewBroker(conf.NatsConn)
	ch := command.NewHandler(conf.NatsConn, cm, &broker)

	chat := &Chat{
		broker:         broker,
		quitCh:         make(chan struct{}),
		channelManager: cm,
		commandHandler: ch,
		connCh:         make(chan websocket.Connection),
		natsConn:       conf.NatsConn,
	}

	router := mux.NewRouter()
	router.Use(auth.ContextMiddleware(conf.Jwt))
	router.Handle("/", chat).Methods("GET")

	srv := websocket.NewServer(router, conf.Addr)
	srv.ListenAndServe()

	chat.wsserver = srv

	return chat
}

func (s *Chat) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := s.wsserver.Upgrade(w, r, http.Header{})
	if err != nil {
		log.WithError(err).Error("upgrading connection")
		return
	}

	s.connCh <- conn
}

func (s *Chat) Start() {
	defer s.close()

	go s.emptyPurger()
	// go s.statsPrinter()

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR1, syscall.SIGUSR2)

	var id int
	for {
		select {
		case conn := <-s.connCh:
			id++
			c, err := client.NewClient(conn, s.commandHandler, id, s.quitCh)
			if err != nil {
				log.WithError(err).Warn("creating new client")
			}

			go c.Serve()
		case <-sigC:
			return
		}
	}
}

func (s *Chat) printStats() {
	memberCount := 0
	channels := s.channelManager.List()

	for _, ch := range channels {
		memberCount += ch.CountMembers()
	}

	log.WithFields(log.Fields{"channels": len(channels), "members": memberCount}).Info("stats")
}

func (s *Chat) close() {
	s.quitOnce.Do(func() { close(s.quitCh) })
	s.wsserver.Close()
	channels := s.channelManager.List()
	for _, ch := range channels {
		ch.Close()
	}
}

func (s *Chat) statsPrinter() {
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

func (s *Chat) emptyPurger() {
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
