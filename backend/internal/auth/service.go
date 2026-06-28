package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/borg-backup-manager/backend/internal/config"
	"github.com/borg-backup-manager/backend/internal/db"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type User struct {
	Sub               string `json:"sub"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
}

type Service struct {
	cfg      *config.Config
	store    *db.Store
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
	metadata map[string]any
	sessions *SessionStore
}

func NewService(cfg *config.Config, store *db.Store) (*Service, error) {
	s := &Service{cfg: cfg, store: store, sessions: &SessionStore{}}
	if err := s.sessions.Init(cfg.SessionDir, cfg.SecretKey); err != nil {
		return nil, err
	}
	if cfg.AuthDisabled || cfg.OIDCClientID == "" {
		return s, nil
	}
	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, issuerFromWellKnown(cfg.OIDCWellKnownEndpoint))
	if err != nil {
		resp, err2 := http.Get(cfg.OIDCWellKnownEndpoint)
		if err2 != nil {
			return s, nil // allow startup without OIDC for dev
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		_ = json.Unmarshal(body, &s.metadata)
		issuer, _ := s.metadata["issuer"].(string)
		if issuer == "" {
			return s, nil
		}
		provider, err = oidc.NewProvider(ctx, issuer)
		if err != nil {
			return s, nil
		}
	}
	s.verifier = provider.Verifier(&oidc.Config{ClientID: cfg.OIDCClientID})
	s.oauth = &oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  "", // set per request
		Scopes:       strings.Fields(cfg.OIDCScope),
	}
	return s, nil
}

func (s *Service) LoginURL(redirectURI, state string) string {
	if s.oauth == nil {
		return "/"
	}
	cfg := *s.oauth
	cfg.RedirectURL = redirectURI
	return cfg.AuthCodeURL(state)
}

func (s *Service) Exchange(ctx context.Context, redirectURI, code string) (*User, error) {
	cfg := *s.oauth
	cfg.RedirectURL = redirectURI
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in response")
	}
	idTok, err := s.verifier.Verify(ctx, rawID)
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := idTok.Claims(&claims); err != nil {
		return nil, err
	}
	return claimsToUser(claims), nil
}

func claimsToUser(claims map[string]any) *User {
	u := &User{}
	if v, ok := claims["sub"].(string); ok {
		u.Sub = v
	}
	if v, ok := claims["email"].(string); ok {
		u.Email = v
	}
	if v, ok := claims["name"].(string); ok {
		u.Name = v
	}
	if v, ok := claims["preferred_username"].(string); ok {
		u.PreferredUsername = v
	}
	if u.PreferredUsername == "" {
		u.PreferredUsername = u.Email
	}
	return u
}

func (s *Service) ExtractGroups(claims map[string]any) map[string]struct{} {
	groups := map[string]struct{}{}
	for _, claimName := range strings.Split(s.cfg.OIDCGroupsClaim, ",") {
		claimName = strings.TrimSpace(claimName)
		if claimName == "" {
			continue
		}
		addGroups(groups, claims[claimName])
	}
	if ra, ok := claims["realm_access"].(map[string]any); ok {
		addGroups(groups, ra["roles"])
	}
	if res, ok := claims["resource_access"].(map[string]any); ok {
		for _, v := range res {
			if m, ok := v.(map[string]any); ok {
				addGroups(groups, m["roles"])
			}
		}
	}
	return groups
}

func addGroups(groups map[string]struct{}, val any) {
	switch v := val.(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				groups[strings.ToLower(strings.Trim(s, "/"))] = struct{}{}
			}
		}
	case []string:
		for _, s := range v {
			if s != "" {
				groups[strings.ToLower(strings.Trim(s, "/"))] = struct{}{}
			}
		}
	case string:
		if v != "" {
			groups[strings.ToLower(strings.Trim(v, "/"))] = struct{}{}
		}
	}
}

func (s *Service) Allowed(groups map[string]struct{}) bool {
	if len(s.cfg.AllowedGroups) == 0 {
		return true
	}
	for g := range groups {
		if _, ok := s.cfg.AllowedGroups[g]; ok {
			return true
		}
	}
	return false
}

func (s *Service) ValidateBearer(ctx context.Context, token string) (*User, error) {
	if s.verifier == nil {
		return nil, fmt.Errorf("bearer auth not configured")
	}
	idTok, err := s.verifier.Verify(ctx, token)
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := idTok.Claims(&claims); err != nil {
		return nil, err
	}
	if !s.Allowed(s.ExtractGroups(claims)) {
		return nil, fmt.Errorf("access denied")
	}
	return claimsToUser(claims), nil
}

func issuerFromWellKnown(endpoint string) string {
	return strings.TrimSuffix(endpoint, "/.well-known/openid-configuration")
}

func HashAPIKey(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

func GenerateAPIKey() (prefix, secret, full string, err error) {
	b := make([]byte, 16)
	if _, err = rand.Read(b); err != nil {
		return
	}
	prefix = hex.EncodeToString(b[:4])
	secret = hex.EncodeToString(b[4:])
	full = prefix + "-" + secret
	return
}

func (s *Service) ValidateAPIKey(ctx context.Context, full string) (*User, error) {
	parts := strings.SplitN(full, "-", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid api key format")
	}
	hash := HashAPIKey(full)
	key, err := s.store.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("api key expired")
	}
	_ = s.store.TouchAPIKey(ctx, key.ID)
	name := "api-key"
	if key.User != nil {
		name = *key.User
	}
	return &User{Sub: fmt.Sprintf("apikey:%d", key.ID), PreferredUsername: name, Name: key.Name}, nil
}

func DevUser() *User {
	return &User{Sub: "dev-user-local", Email: "dev@localhost", Name: "Dev User", PreferredUsername: "dev"}
}

func (s *Service) Sessions() *SessionStore {
	return s.sessions
}
