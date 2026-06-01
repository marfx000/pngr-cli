package client

import (
	"context"
	"net/http"
	"time"
)

type AuthService struct{ c *Client }

// User mirrors the API's user response. PendingEmailChange is only populated by
// the PATCH /me handler when an email change was requested.
type User struct {
	ID                 string    `json:"id"`
	Email              string    `json:"email"`
	DisplayName        string    `json:"display_name"`
	EmailVerified      bool      `json:"email_verified"`
	CreatedAt          time.Time `json:"created_at"`
	PendingEmailChange *string   `json:"pending_email_change,omitempty"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type LoginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me,omitempty"`
}

// Register creates a new user. The server sends a verification email; the user
// must call VerifyEmail before they can log in.
func (s *AuthService) Register(ctx context.Context, req RegisterRequest) (*User, error) {
	var out User
	if err := s.c.doJSON(ctx, http.MethodPost, "/auth/register", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Login exchanges credentials for an access token (refresh token is set as an
// HttpOnly cookie when called from a browser context; for CLI/MCP we rely on
// the access token in the response).
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*TokenResponse, error) {
	var out TokenResponse
	if err := s.c.doJSON(ctx, http.MethodPost, "/auth/login", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Logout revokes the current refresh token. The caller is responsible for
// discarding the access token client-side.
func (s *AuthService) Logout(ctx context.Context) error {
	return s.c.doJSON(ctx, http.MethodPost, "/auth/logout", nil, nil, nil)
}

// Refresh requires the refresh_token cookie to be present; CLI / non-browser
// clients typically don't have one. Included for completeness.
func (s *AuthService) Refresh(ctx context.Context) (*TokenResponse, error) {
	var out TokenResponse
	if err := s.c.doJSON(ctx, http.MethodPost, "/auth/refresh", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type VerifyEmailRequest struct {
	Token string `json:"token"`
}

// VerifyEmail consumes a one-time token (from the registration / email-change
// email) and atomically flips email_verified to true. Returns the freshly
// loaded user.
func (s *AuthService) VerifyEmail(ctx context.Context, token string) (*User, error) {
	var out User
	if err := s.c.doJSON(ctx, http.MethodPost, "/auth/verify-email", nil,
		VerifyEmailRequest{Token: token}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type ResendVerificationRequest struct {
	Email string `json:"email"`
}

// ResendVerification requests a fresh verification email. The server returns
// 204 regardless of whether the email exists (anti-enumeration).
func (s *AuthService) ResendVerification(ctx context.Context, email string) error {
	return s.c.doJSON(ctx, http.MethodPost, "/auth/resend-verification", nil,
		ResendVerificationRequest{Email: email}, nil)
}

// OAuthAuthorizeURL returns the URL a browser should visit to start an OAuth
// flow for the given provider ("github" | "google"). Not a network call —
// the URL is computed locally from the client's baseURL.
func (s *AuthService) OAuthAuthorizeURL(provider string) string {
	return s.c.baseURL + "/auth/oauth/" + provider
}
