package client

import (
	"context"
	"net/http"
)

type MeService struct{ c *Client }

type UpdateMeRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	// Email change: the new address is NOT applied directly. The server sends
	// a verification email and only swaps users.email when the link is clicked.
	// The response's PendingEmailChange field carries the new address.
	Email *string `json:"email,omitempty"`
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Get returns the currently authenticated user.
func (s *MeService) Get(ctx context.Context) (*User, error) {
	var out User
	if err := s.c.doJSON(ctx, http.MethodGet, "/me", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update changes display_name immediately. An email change triggers a
// verification flow; see PendingEmailChange in the returned User.
func (s *MeService) Update(ctx context.Context, req UpdateMeRequest) (*User, error) {
	var out User
	if err := s.c.doJSON(ctx, http.MethodPatch, "/me", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdatePassword changes the user's password. Requires the current password
// for verification. Returns nil on success.
func (s *MeService) UpdatePassword(ctx context.Context, req UpdatePasswordRequest) error {
	return s.c.doJSON(ctx, http.MethodPatch, "/me/password", nil, req, nil)
}

// Delete soft-deletes the current user account (30-day recovery window).
func (s *MeService) Delete(ctx context.Context) error {
	return s.c.doJSON(ctx, http.MethodDelete, "/me", nil, nil, nil)
}
