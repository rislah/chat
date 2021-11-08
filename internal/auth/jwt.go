package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const (
	expiresIn = 24 * time.Hour
)

var (
	ErrJWTAlgMismatch             = errors.New("token algorithm mismatch")
	ErrJWTInvalid                 = errors.New("invalid token provided")
	ErrJWTExpired                 = errors.New("expired token")
	ErrJWTUnknownSigningAlgorithm = errors.New("unknown signing algorithm")
)

type UserClaims struct {
	*jwt.RegisteredClaims
	Username string `json:"username"`
	Role     string `json:"role"`
}

func NewRegisteredClaims(expiresIn time.Duration) jwt.RegisteredClaims {
	now := time.Now()
	return jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(expiresIn)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
}

func NewUserClaims(username string, role string) UserClaims {
	rc := NewRegisteredClaims(expiresIn)
	uc := UserClaims{
		RegisteredClaims: &rc,
		Username:         username,
		Role:             role,
	}
	return uc
}

type JWTWrapper struct {
	Algorithm jwt.SigningMethod
	Secret    string
}

func NewHS256Wrapper(secret string) JWTWrapper {
	return JWTWrapper{
		Algorithm: jwt.SigningMethodHS256,
		Secret:    secret,
	}
}

func (w JWTWrapper) Encode(claims jwt.Claims) (string, error) {
	switch w.Algorithm {
	case jwt.SigningMethodHS256:
		token := jwt.NewWithClaims(w.Algorithm, claims)
		return token.SignedString([]byte(w.Secret))
	default:
		return "", ErrJWTUnknownSigningAlgorithm
	}
}

func (w JWTWrapper) Decode(tokenStr string, claims jwt.Claims) (*jwt.Token, error) {
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(w.Secret), nil
	})

	if err != nil {
		if e, ok := err.(*jwt.ValidationError); ok {
			if e.Errors == jwt.ValidationErrorExpired {
				return nil, ErrJWTExpired
			}
		}
		return nil, err
	}

	if !token.Valid {
		return nil, ErrJWTInvalid
	}

	if token.Method.Alg() != w.Algorithm.Alg() {
		return nil, ErrJWTAlgMismatch
	}

	return token, nil
}
