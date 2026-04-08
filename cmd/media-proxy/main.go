package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	authorizationv1 "github.com/agynio/media-proxy/.gen/go/agynio/api/authorization/v1"
	filesv1 "github.com/agynio/media-proxy/.gen/go/agynio/api/files/v1"
	usersv1 "github.com/agynio/media-proxy/.gen/go/agynio/api/users/v1"
	"github.com/agynio/media-proxy/internal/auth"
	"github.com/agynio/media-proxy/internal/config"
	"github.com/agynio/media-proxy/internal/grpcclient"
	"github.com/agynio/media-proxy/internal/proxy"
	"github.com/rs/cors"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	verifier, err := auth.NewVerifier(ctx, cfg.OIDCIssuerURL, cfg.OIDCClientID)
	if err != nil {
		log.Fatalf("oidc verifier error: %v", err)
	}

	usersClient, err := grpcclient.New(cfg.UsersGRPCTarget, usersv1.NewUsersServiceClient)
	if err != nil {
		log.Fatalf("users client error: %v", err)
	}
	defer func() {
		if err := usersClient.Close(); err != nil {
			log.Printf("users client close error: %v", err)
		}
	}()

	filesClient, err := grpcclient.New(cfg.FilesGRPCTarget, filesv1.NewFilesServiceClient)
	if err != nil {
		log.Fatalf("files client error: %v", err)
	}
	defer func() {
		if err := filesClient.Close(); err != nil {
			log.Printf("files client close error: %v", err)
		}
	}()

	authzClient, err := grpcclient.New(cfg.AuthzGRPCTarget, authorizationv1.NewAuthorizationServiceClient)
	if err != nil {
		log.Fatalf("authorization client error: %v", err)
	}
	defer func() {
		if err := authzClient.Close(); err != nil {
			log.Printf("authorization client close error: %v", err)
		}
	}()

	resolver, err := auth.NewResolver(verifier, usersClient.Service(), &http.Client{Timeout: 10 * time.Second})
	if err != nil {
		log.Fatalf("auth resolver error: %v", err)
	}

	handler, err := proxy.NewHandler(cfg, resolver, filesClient.Service(), authzClient.Service())
	if err != nil {
		log.Fatalf("proxy handler error: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/proxy", handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := newCORSHandler(cfg, mux)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           corsHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("server error: %v", err)
	}
}

func newCORSHandler(cfg config.Config, handler http.Handler) http.Handler {
	return cors.New(cors.Options{
		AllowedOrigins:   []string{cfg.CORSAllowedOrigin},
		AllowedMethods:   []string{http.MethodGet, http.MethodOptions},
		AllowedHeaders:   []string{"Authorization", "Range"},
		ExposedHeaders:   []string{"Content-Type", "Content-Range", "Accept-Ranges", "Content-Disposition", "Content-Length"},
		AllowCredentials: true,
		MaxAge:           86400,
	}).Handler(handler)
}
