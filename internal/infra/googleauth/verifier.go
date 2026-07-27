// Package googleauth verifies Google ID Tokens. It's infra: it doesn't
// know about our domain, our database, or our own JWTs — only about
// asking Google's library "is this token real, and who is it for".
package googleauth

import (
	"context"
	"fmt"

	"google.golang.org/api/idtoken"
)

// Claims is what we pull out of a verified Google ID Token — just enough
// to find or create the matching user.
type Claims struct {
	Sub   string // Google's stable, unique ID for this user.
	Email string
	Name  string
}

type Verifier struct {
	clientID string
}

// NewVerifier takes our app's Google OAuth Client ID. Every token we
// verify must have been issued for this exact client ID (the `aud`
// claim) — that's what stops a token meant for a different app being
// replayed against ours.
func NewVerifier(clientID string) *Verifier {
	return &Verifier{clientID: clientID}
}

func (v *Verifier) Verify(ctx context.Context, rawToken string) (Claims, error) {
	payload, err := idtoken.Validate(ctx, rawToken, v.clientID)
	if err != nil {
		return Claims{}, fmt.Errorf("verify google id token: %w", err)
	}

	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)

	return Claims{
		Sub:   payload.Subject,
		Email: email,
		Name:  name,
	}, nil
}
