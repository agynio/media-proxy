package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	usersv1 "github.com/agynio/media-proxy/.gen/go/agynio/api/users/v1"
	"github.com/agynio/media-proxy/internal/identity"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Resolver struct {
	verifier         *Verifier
	usersClient      usersv1.UsersServiceClient
	userinfoEndpoint string
	httpClient       *http.Client
}

func NewResolver(verifier *Verifier, usersClient usersv1.UsersServiceClient, httpClient *http.Client) (*Resolver, error) {
	if verifier == nil {
		return nil, fmt.Errorf("verifier is required")
	}
	if usersClient == nil {
		return nil, fmt.Errorf("users client is required")
	}
	if httpClient == nil {
		return nil, fmt.Errorf("http client is required")
	}
	userinfoEndpoint := strings.TrimSpace(verifier.UserinfoEndpoint())
	if userinfoEndpoint == "" {
		return nil, fmt.Errorf("userinfo endpoint is required")
	}

	return &Resolver{
		verifier:         verifier,
		usersClient:      usersClient,
		userinfoEndpoint: userinfoEndpoint,
		httpClient:       httpClient,
	}, nil
}

func (r *Resolver) ResolveFromToken(ctx context.Context, accessToken string) (identity.ResolvedIdentity, error) {
	claims, err := r.verifier.Verify(ctx, accessToken)
	if err != nil {
		return identity.ResolvedIdentity{}, status.Errorf(codes.Unauthenticated, "invalid bearer token: %v", err)
	}

	getResponse, err := r.usersClient.GetUserByOIDCSubject(ctx, &usersv1.GetUserByOIDCSubjectRequest{
		OidcSubject: claims.Subject,
	})
	if err == nil {
		return identityFromUser(getResponse.GetUser())
	}
	if status.Code(err) != codes.NotFound {
		return identity.ResolvedIdentity{}, err
	}

	userInfo, err := r.fetchUserInfo(ctx, accessToken, claims.Subject)
	if err != nil {
		return identity.ResolvedIdentity{}, status.Errorf(codes.Internal, "failed to fetch user info: %v", err)
	}

	createResponse, err := r.usersClient.ResolveOrCreateUser(ctx, &usersv1.ResolveOrCreateUserRequest{
		OidcSubject: userInfo.Subject,
		Name:        userInfo.Name,
		Email:       userInfo.Email,
		PhotoUrl:    userInfo.Picture,
	})
	if err != nil {
		return identity.ResolvedIdentity{}, err
	}

	return identityFromUser(createResponse.GetUser())
}

func (r *Resolver) fetchUserInfo(ctx context.Context, accessToken, expectedSub string) (*oidc.UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.userinfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	const maxUserInfoResponseBytes = 1 << 20
	limitedBody := io.LimitReader(resp.Body, maxUserInfoResponseBytes)

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(limitedBody)
		if readErr != nil {
			return nil, fmt.Errorf("userinfo request failed with status %s", resp.Status)
		}
		return nil, fmt.Errorf("userinfo request failed with status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var userInfo oidc.UserInfo
	decoder := json.NewDecoder(limitedBody)
	if err := decoder.Decode(&userInfo); err != nil {
		return nil, err
	}

	userInfo.Subject = strings.TrimSpace(userInfo.Subject)
	if userInfo.Subject == "" {
		return nil, fmt.Errorf("userinfo subject is required")
	}
	if userInfo.Subject != expectedSub {
		return nil, fmt.Errorf("userinfo subject does not match expected subject")
	}
	userInfo.Name = strings.TrimSpace(userInfo.Name)
	userInfo.Email = strings.TrimSpace(userInfo.Email)
	userInfo.Picture = strings.TrimSpace(userInfo.Picture)

	return &userInfo, nil
}

func identityFromUser(user *usersv1.User) (identity.ResolvedIdentity, error) {
	if user == nil {
		return identity.ResolvedIdentity{}, fmt.Errorf("user missing from response")
	}
	meta := user.GetMeta()
	if meta == nil {
		return identity.ResolvedIdentity{}, fmt.Errorf("user metadata missing from response")
	}
	identityID := strings.TrimSpace(meta.GetId())
	if identityID == "" {
		return identity.ResolvedIdentity{}, fmt.Errorf("identity id missing")
	}

	return identity.ResolvedIdentity{
		IdentityID:   identityID,
		IdentityType: identity.IdentityTypeUser,
	}, nil
}
