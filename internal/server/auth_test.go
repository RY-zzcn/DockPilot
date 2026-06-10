package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPasswordHashAndToken(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !VerifyPassword("secret", hash) {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatal("wrong password verified")
	}

	auth := NewAuthService("test-secret")
	token, err := auth.Issue(User{ID: 7, Username: "admin", Role: RoleAdmin})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	claims, err := auth.Parse(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Username != "admin" || claims.Role != RoleAdmin {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestLoadConfigReplacesInsecureAuthSecret(t *testing.T) {
	t.Setenv("DOCKPILOT_AUTH_SECRET", insecureDefaultAuthSecret)
	cfg := LoadConfig()
	if cfg.AuthSecret == "" || cfg.AuthSecret == insecureDefaultAuthSecret {
		t.Fatalf("expected generated auth secret, got %q", cfg.AuthSecret)
	}

	t.Setenv("DOCKPILOT_AUTH_SECRET", "custom-secret")
	cfg = LoadConfig()
	if cfg.AuthSecret != "custom-secret" {
		t.Fatalf("expected custom auth secret to be preserved, got %q", cfg.AuthSecret)
	}
}

func TestAuthenticatedMiddlewareRequiresExistingUser(t *testing.T) {
	store := testStore(t)
	user, err := store.CreateUser("admin", "secret", RoleAdmin)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	app := &App{store: store, auth: NewAuthService("test-secret")}
	token, err := app.auth.Issue(user)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	handler := app.authenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("valid token returned %d: %s", rec.Code, rec.Body.String())
	}

	staleToken, err := app.auth.Issue(User{ID: user.ID + 100, Username: "admin", Role: RoleAdmin})
	if err != nil {
		t.Fatalf("issue stale token: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+staleToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("stale token returned %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
