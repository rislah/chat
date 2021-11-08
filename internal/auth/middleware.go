package auth

import (
	"chat/internal/ctxkeys"
	"context"
	"net/http"
	"reflect"
	"strings"

	"github.com/sirupsen/logrus"
)

type Context struct {
	Username string
	Role     string
}

func ContextMiddleware(jwt JWTWrapper) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			token, found := extractJWT(r)
			if !found {
				next.ServeHTTP(rw, r)
				return
			}

			claimsToken, err := jwt.Decode(token, &UserClaims{})
			if err != nil {
				logrus.WithError(err).Warn("decoding jwt")
				return
			}

			claims, ok := claimsToken.Claims.(*UserClaims)
			if !ok {
				logrus.WithField("type", reflect.TypeOf(claimsToken.Claims)).Warn("casting to userclaims")
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxkeys.User, &Context{Username: claims.Username, Role: claims.Role})

			next.ServeHTTP(rw, r.WithContext(ctx))
		})
	}
}

func extractJWT(r *http.Request) (string, bool) {
	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		return "", false
	}

	token := strings.TrimPrefix(authorization, "Bearer")
	if token == "" {
		return "", false
	}

	return strings.TrimSpace(token), true
}
