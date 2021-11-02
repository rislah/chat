package websocket

import (
	"context"
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

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Connection
type Connection interface {
	ReadJSON(v interface{}) error
	WriteJSON(v interface{}) error
	Context() context.Context
	Close() error
}

type upgradedConnection struct {
	conn *websocket.Conn
	ctx  context.Context
}

var _ Connection = &upgradedConnection{}

func NewUpgradedConnection(conn *websocket.Conn, ctx context.Context) *upgradedConnection {
	return &upgradedConnection{conn: conn, ctx: ctx}
}

func (uc *upgradedConnection) ReadJSON(v interface{}) error {
	err := uc.conn.ReadJSON(v)
	if err != nil {
		return err
	}

	return nil
}

func (uc *upgradedConnection) WriteJSON(v interface{}) error {
	if err := uc.conn.WriteJSON(v); err != nil {
		return err
	}

	return nil
}

func (uc *upgradedConnection) Context() context.Context {
	return uc.ctx
}

func (uc *upgradedConnection) Close() error {
	return uc.conn.Close()
}

type Server interface {
	Close()
	Upgrade(rw http.ResponseWriter, r *http.Request, header http.Header) (Connection, error)
	Serve()
	AcceptConn() Connection
}

type websocketServerImpl struct {
	mux             *mux.Router
	addr            string
	upgrader        *websocket.Upgrader
	upgradedConnsCh chan Connection
	listener        net.Listener
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

	// upgradedConnsCh := make(chan Connection)

	// upgraderHandler := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
	// 	fmt.Println(r.Header)
	// 	conn, err := upgrader.Upgrade(rw, r, http.Header{})
	// 	if err != nil {
	// 		log.Println(err)
	// 		return
	// 	}

	// 	upgradedConnsCh <- NewUpgradedConnection(conn)
	// })

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

func (ws *websocketServerImpl) AcceptConn() Connection {
	for {
		select {
		case conn := <-ws.upgradedConnsCh:
			return conn
		}
	}
}

func (ws *websocketServerImpl) Close() {
	ws.listener.Close()
}
