package server

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type contextKey string

const userContextKey contextKey = "dockpilot-user"

type AuthClaims struct {
	Subject  string `json:"sub"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Expires  int64  `json:"exp"`
}

type AuthService struct {
	secret []byte
}

func NewAuthService(secret string) *AuthService {
	return &AuthService{secret: []byte(secret)}
}

func (a *AuthService) Issue(user User) (string, error) {
	claims := AuthClaims{
		Subject:  fmt.Sprintf("%d", user.ID),
		Username: user.Username,
		Role:     user.Role,
		Expires:  time.Now().Add(24 * time.Hour).Unix(),
	}
	raw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	sig := sign(body, a.secret)
	return body + "." + sig, nil
}

func (a *AuthService) Parse(token string) (AuthClaims, error) {
	var claims AuthClaims
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return claims, errors.New("invalid token")
	}
	if !secureEqual(sign(parts[0], a.secret), parts[1]) {
		return claims, errors.New("invalid signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return claims, err
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return claims, err
	}
	if time.Now().Unix() > claims.Expires {
		return claims, errors.New("token expired")
	}
	return claims, nil
}

func (a *AuthService) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		claims, err := a.Parse(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(userContextKey).(AuthClaims)
		if !ok || claims.Role != RoleAdmin {
			writeError(w, http.StatusForbidden, "admin role required")
			return
		}
		next(w, r)
	}
}

func CurrentUser(r *http.Request) AuthClaims {
	claims, _ := r.Context().Value(userContextKey).(AuthClaims)
	return claims
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	iterations := 120000
	sum := passwordDigest(password, salt, iterations)
	return fmt.Sprintf("sha256$%d$%s$%s", iterations, hex.EncodeToString(salt), hex.EncodeToString(sum)), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "sha256" {
		return false
	}
	var iterations int
	if _, err := fmt.Sscanf(parts[1], "%d", &iterations); err != nil {
		return false
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}
	actual := passwordDigest(password, salt, iterations)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func passwordDigest(password string, salt []byte, iterations int) []byte {
	sum := sha256.Sum256(append(salt, []byte(password)...))
	current := sum[:]
	for i := 1; i < iterations; i++ {
		next := sha256.Sum256(append(current, []byte(password)...))
		current = next[:]
	}
	out := make([]byte, len(current))
	copy(out, current)
	return out
}

func sign(body string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func secureEqual(a, b string) bool {
	ab, errA := base64.RawURLEncoding.DecodeString(a)
	bb, errB := base64.RawURLEncoding.DecodeString(b)
	if errA != nil || errB != nil {
		return false
	}
	return subtle.ConstantTimeCompare(ab, bb) == 1
}

func RandomToken(prefix string) string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return prefix + hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return prefix + hex.EncodeToString(b)
}
