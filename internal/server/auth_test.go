package server

import "testing"

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
