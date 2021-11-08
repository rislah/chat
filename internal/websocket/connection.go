package websocket

import (
	"context"

	"github.com/gorilla/websocket"
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
