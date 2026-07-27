// Package authservice orchestrates the login use case: verify a Google ID
// Token, resolve it to our own user record, issue our own JWT. Each step
// is a dependency defined here by the consumer, so this package doesn't
// know it's actually talking to Google, Postgres, or golang-jwt.
package authservice

import (
	"context"
	"errors"
	"fmt"

	"github.com/Cortalo/digitalgarden-backend/internal/infra/googleauth"
	userservice "github.com/Cortalo/digitalgarden-backend/internal/service/user"
)

// ErrUnauthorized means the caller's fault: the Google token itself was
// invalid or expired. Any other error from LoginWithGoogle is ours (DB,
// token signing, etc.) and shouldn't be reported to the client as a bad
// login attempt.
var ErrUnauthorized = errors.New("authservice: invalid google token")

type GoogleVerifier interface {
	Verify(ctx context.Context, rawToken string) (googleauth.Claims, error)
}

type UserFinder interface {
	FindOrCreateByGoogle(ctx context.Context, googleSub, email, name string) (userservice.User, error)
}

type TokenIssuer interface {
	Issue(userID int64) (string, error)
}

type Service struct {
	google GoogleVerifier
	users  UserFinder
	tokens TokenIssuer
}

func NewService(google GoogleVerifier, users UserFinder, tokens TokenIssuer) *Service {
	return &Service{google: google, users: users, tokens: tokens}
}

// LoginWithGoogle verifies rawGoogleToken, finds or creates the matching
// user, and returns our own signed JWT for them.
func (s *Service) LoginWithGoogle(ctx context.Context, rawGoogleToken string) (string, error) {
	claims, err := s.google.Verify(ctx, rawGoogleToken)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnauthorized, err)
	}

	u, err := s.users.FindOrCreateByGoogle(ctx, claims.Sub, claims.Email, claims.Name)
	if err != nil {
		return "", fmt.Errorf("find or create user: %w", err)
	}

	token, err := s.tokens.Issue(u.ID)
	if err != nil {
		return "", fmt.Errorf("issue token: %w", err)
	}

	return token, nil
}
