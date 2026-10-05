package socialauth

import (
	"context"

	"github.com/warmbly/warmbly/internal/pkg/appleauth"
)

// stubAppleClient stands in for the Apple token endpoint. Only construction and
// URL building are exercised here; the exchange needs Apple's live keys.
type stubAppleClient struct{}

func (stubAppleClient) ExchangeCode(context.Context, string, string) (*appleauth.TokenResponse, error) {
	return nil, nil
}
