package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	httphelper "github.com/zitadel/oidc/v3/pkg/http"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

type Claims struct {
	Subject string
}

type Verifier struct {
	issuer            string
	clientID          string
	keySet            oidc.KeySet
	userinfoEndpoint  string
	supportedSignAlgs []string // nil accepts any JWKS alg (matches gateway verifier).
	clockSkew         time.Duration
}

func NewVerifier(ctx context.Context, issuer, clientID string) (*Verifier, error) {
	trimmedIssuer := strings.TrimSpace(issuer)
	if trimmedIssuer == "" {
		return nil, fmt.Errorf("issuer is required")
	}
	trimmedClientID := strings.TrimSpace(clientID)
	if trimmedClientID == "" {
		return nil, fmt.Errorf("client id is required")
	}

	discovery, err := client.Discover(ctx, trimmedIssuer, httphelper.DefaultHTTPClient)
	if err != nil {
		return nil, err
	}
	jwksURI := strings.TrimSpace(discovery.JwksURI)
	if jwksURI == "" {
		return nil, fmt.Errorf("jwks uri missing from discovery")
	}
	userinfoEndpoint := strings.TrimSpace(discovery.UserinfoEndpoint)

	return &Verifier{
		issuer:           trimmedIssuer,
		clientID:         trimmedClientID,
		keySet:           rp.NewRemoteKeySet(httphelper.DefaultHTTPClient, jwksURI),
		userinfoEndpoint: userinfoEndpoint,
		clockSkew:        time.Second,
	}, nil
}

func (v *Verifier) UserinfoEndpoint() string {
	return v.userinfoEndpoint
}

type tokenClaims struct {
	oidc.TokenClaims
}

func (v *Verifier) Verify(ctx context.Context, accessToken string) (Claims, error) {
	decrypted, err := oidc.DecryptToken(accessToken)
	if err != nil {
		return Claims{}, err
	}

	var parsed tokenClaims
	payload, err := oidc.ParseToken(decrypted, &parsed)
	if err != nil {
		return Claims{}, err
	}

	if err := oidc.CheckSubject(&parsed); err != nil {
		return Claims{}, err
	}
	if err := oidc.CheckIssuer(&parsed, v.issuer); err != nil {
		return Claims{}, err
	}
	if err := oidc.CheckSignature(ctx, decrypted, payload, &parsed, v.supportedSignAlgs, v.keySet); err != nil {
		return Claims{}, err
	}
	if err := oidc.CheckExpiration(&parsed, v.clockSkew); err != nil {
		return Claims{}, err
	}

	subject := strings.TrimSpace(parsed.Subject)
	if subject == "" {
		return Claims{}, fmt.Errorf("subject claim is required")
	}

	return Claims{
		Subject: subject,
	}, nil
}
