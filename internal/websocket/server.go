package websocket

import (
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	readBufferSize  = 1024
	writeBufferSize = 1024
)

type Server interface {
	Close()
	Upgrade(rw http.ResponseWriter, r *http.Request, header http.Header) (Connection, error)
	Serve()
}

type websocketServerImpl struct {
	mux      *mux.Router
	addr     string
	upgrader *websocket.Upgrader
	listener net.Listener
}

var _ Server = &websocketServerImpl{}

func NewServer(mux *mux.Router, addr string) *websocketServerImpl {
	upgrader := &websocket.Upgrader{
		HandshakeTimeout:  10 * time.Second,
		ReadBufferSize:    readBufferSize,
		WriteBufferSize:   writeBufferSize,
		EnableCompression: true,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return &websocketServerImpl{
		upgrader: upgrader,
		addr:     addr,
		mux:      mux,
	}
}

func (ws *websocketServerImpl) Serve() {
	listener, err := net.Listen("tcp", ws.addr)
	if err != nil {
		log.Fatal(err)
	}

	ws.listener = listener

	go func() {
		if err := http.Serve(listener, ws.mux); err != nil {
			log.Println(err)
			return
		}
	}()
}

func (ws *websocketServerImpl) Upgrade(rw http.ResponseWriter, r *http.Request, header http.Header) (Connection, error) {
	conn, err := ws.upgrader.Upgrade(rw, r, header)
	if err != nil {
		return nil, err
	}
	return NewUpgradedConnection(conn, r.Context()), nil
}

func (ws *websocketServerImpl) Close() {
	ws.listener.Close()
}
