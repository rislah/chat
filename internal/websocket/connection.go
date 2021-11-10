package websocket

import (
	"context"
	"sync"

	"github.com/gorilla/websocket"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Connection
type Connection interface {
	Read() ([]byte, error)
	WriteJSON(v interface{}) error
	Context() context.Context
	Close()
}

type upgradedConnection struct {
	conn      *websocket.Conn
	ctx       context.Context
	closeOnce sync.Once
}

var _ Connection = &upgradedConnection{}

func NewUpgradedConnection(conn *websocket.Conn, ctx context.Context) *upgradedConnection {
	return &upgradedConnection{conn: conn, ctx: ctx}
}

func (uc *upgradedConnection) WriteJSON(v interface{}) error {
	return uc.conn.WriteJSON(v)
}

func (uc *upgradedConnection) Context() context.Context {
	return uc.ctx
}

func (uc *upgradedConnection) Close() {
	uc.closeOnce.Do(func() {
		_ = uc.conn.Close()
	})
}

func (uc *upgradedConnection) Read() ([]byte, error) {
	messageType, data, err := uc.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	if messageType != websocket.TextMessage {
		return nil, nil
	}

	return data, nil
}
