package client

import (
	"chat/internal/auth"
	"chat/internal/ctxkeys"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

type user struct {
	username string
	guest    bool
}

func (u *user) generateGuestName() (string, error) {
	id := make([]byte, 2)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		return "", err
	}
	return fmt.Sprintf("guest%x", id), nil
}

func (u *user) newFromCtx(ctx context.Context) error {
	val := ctx.Value(ctxkeys.User)
	if val != nil {
		a, ok := val.(*auth.Context)
		if !ok {
			// should never ever happen
			return errors.New("failed to assert context value")
		}

		u.username = a.Username
		u.guest = false

		return nil
	}

	name, err := u.generateGuestName()
	if err != nil {
		return err
	}

	u.username = name
	u.guest = true

	return nil
}
