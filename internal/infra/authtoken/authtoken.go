// Package authtoken issues and verifies our own JWTs. This is separate
// from googleauth: googleauth verifies tokens someone else (Google)
// signed, this package signs and verifies tokens we ourselves issue.
package authtoken

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("authtoken: invalid token")

const expiry = 7 * 24 * time.Hour

// Issuer signs tokens with, and verifies tokens against, a single shared
// secret (HS256) — we're both the issuer and the verifier, so there's no
// need for asymmetric keys.
type Issuer struct {
	secret []byte
}

func NewIssuer(secret string) *Issuer {
	return &Issuer{secret: []byte(secret)}
}

// Issue signs a token whose only real payload is the user ID (as the
// standard "sub" claim), plus issued-at/expiry.
func (i *Issuer) Issue(userID int64) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return signed, nil
}

// Verify checks the signature and expiry, then returns the user ID from
// the subject claim. It pins the signing method to HMAC so a token can't
// smuggle in a different algorithm (e.g. "none") to bypass verification.
func (i *Issuer) Verify(tokenString string) (int64, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return i.secret, nil
	})
	if err != nil || !token.Valid {
		return 0, ErrInvalidToken
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return 0, ErrInvalidToken
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, ErrInvalidToken
	}

	return userID, nil
}
