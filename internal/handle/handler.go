package handler

import (
	"chat/internal/channel"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"chat/internal/client"
	"chat/internal/keys"
	"chat/internal/websocket"

	"github.com/gorilla/mux"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type Server struct {
	wsserver       websocket.Server
	channelManager *channel.Manager
	connCh         chan websocket.Connection
	connChOnce     sync.Once
	natsConn       *nats.Conn
	quitCh         chan struct{}
	signalCh       chan os.Signal
	quitChOnce     sync.Once
}

func NewServer(addr string, path string, natsConn *nats.Conn) *Server {
	srv := &Server{
		channelManager: channel.NewChannelsManager(natsConn),
		natsConn:       natsConn,
		connCh:         make(chan websocket.Connection),
		quitCh:         make(chan struct{}),
		signalCh:       make(chan os.Signal),
	}
	signal.Notify(srv.signalCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR1, syscall.SIGUSR2)

	router := mux.NewRouter()
	router.Use(contextMiddleware)
	router.HandleFunc(path, srv.handler).Methods("GET")

	wsserver := websocket.NewServer(router, addr, srv.handler)
	srv.wsserver = wsserver
	return srv
}

func contextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		var (
			username = r.Header.Get("username")
			guest    = r.Header.Get("guest")
		)

		ctx := r.Context()
		ctx = context.WithValue(ctx, keys.UsernameCtx, username)
		ctx = context.WithValue(ctx, keys.GuestCtx, guest)

		next.ServeHTTP(rw, r.WithContext(ctx))
	})
}

func (s *Server) close() {
	s.quitChOnce.Do(func() { close(s.quitCh) })
	s.connChOnce.Do(func() { close(s.connCh) })
	s.wsserver.Close()

	channels := s.channelManager.List()
	for _, channel := range channels {
		channel.Close()
	}
}

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	conn, err := s.wsserver.Upgrade(w, r, http.Header{})
	if err != nil {
		zap.L().Warn("upgrading connection", zap.Error(err))
		return
	}

	s.connCh <- conn
}

func (s *Server) printStats() {
	memberCount := 0
	channels := s.channelManager.List()
	for _, channel := range channels {
		memberCount += channel.CountMembers()
	}

	zap.L().Info(fmt.Sprintf("%d channels with total %d members", len(channels), memberCount))
}

func (s *Server) Start() {
	defer s.close()

	go s.wsserver.Serve()
	go s.statsPrinter()
	go s.emptyChannelPurger()
	clientID := 0

	for {
		select {
		case conn := <-s.connCh:
			clientID++
			client := client.NewClient(conn, s.channelManager, s.natsConn, clientID, s.quitCh)
			go client.Serve()
		case <-s.signalCh:
			return
		}
	}
}

func (s *Server) statsPrinter() {
	ticker := time.NewTicker(6 * time.Second)
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

func (s *Server) emptyChannelPurger() {
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
