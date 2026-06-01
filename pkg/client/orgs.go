package client

import (
	"context"
	"net/http"
	"time"
)

type OrgsService struct{ c *Client }

type Org struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	LogoURL   string    `json:"logo_url"`
	CreatedAt time.Time `json:"created_at"`
}

type Member struct {
	UserID      string    `json:"user_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"` // "owner" | "admin" | "member" | "viewer"
	JoinedAt    time.Time `json:"joined_at"`
}

type Invitation struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Token     string    `json:"token"` // omitted from API once email delivery lands in Phase 3
	ExpiresAt time.Time `json:"expires_at"`
}

type CreateOrgRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug,omitempty"`
}

type UpdateOrgRequest struct {
	Name    *string `json:"name,omitempty"`
	Slug    *string `json:"slug,omitempty"`
	LogoURL *string `json:"logo_url,omitempty"`
}

type InviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type AcceptInvitationRequest struct {
	Token string `json:"token"`
}

type UpdateRoleRequest struct {
	Role string `json:"role"`
}

type TransferOwnershipRequest struct {
	UserID string `json:"user_id"`
}

// ─── orgs ─────────────────────────────────────────────────────────────────────

// List returns all organizations the authenticated user belongs to.
func (s *OrgsService) List(ctx context.Context) ([]Org, error) {
	var out []Org
	if err := s.c.doJSON(ctx, http.MethodGet, "/orgs", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create makes a new org with the caller as Owner. Slug auto-generates if empty.
func (s *OrgsService) Create(ctx context.Context, req CreateOrgRequest) (*Org, error) {
	var out Org
	if err := s.c.doJSON(ctx, http.MethodPost, "/orgs", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *OrgsService) Get(ctx context.Context, orgID string) (*Org, error) {
	var out Org
	if err := s.c.doJSON(ctx, http.MethodGet, "/orgs/"+orgID, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *OrgsService) Update(ctx context.Context, orgID string, req UpdateOrgRequest) (*Org, error) {
	var out Org
	if err := s.c.doJSON(ctx, http.MethodPatch, "/orgs/"+orgID, nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete soft-deletes the org. Owner-only on the server side.
func (s *OrgsService) Delete(ctx context.Context, orgID string) error {
	return s.c.doJSON(ctx, http.MethodDelete, "/orgs/"+orgID, nil, nil, nil)
}

// ─── members ──────────────────────────────────────────────────────────────────

func (s *OrgsService) ListMembers(ctx context.Context, orgID string) ([]Member, error) {
	var out []Member
	if err := s.c.doJSON(ctx, http.MethodGet, "/orgs/"+orgID+"/members", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveMember kicks a user from the org. Admin-only on the server.
func (s *OrgsService) RemoveMember(ctx context.Context, orgID, userID string) error {
	return s.c.doJSON(ctx, http.MethodDelete, "/orgs/"+orgID+"/members/"+userID, nil, nil, nil)
}

// UpdateRole changes a member's role.
func (s *OrgsService) UpdateRole(ctx context.Context, orgID, userID, role string) error {
	return s.c.doJSON(ctx, http.MethodPatch, "/orgs/"+orgID+"/members/"+userID+"/role", nil,
		UpdateRoleRequest{Role: role}, nil)
}

// Leave removes the caller's membership. Last-owner returns CANNOT_LEAVE_AS_OWNER.
func (s *OrgsService) Leave(ctx context.Context, orgID string) error {
	return s.c.doJSON(ctx, http.MethodDelete, "/orgs/"+orgID+"/membership", nil, nil, nil)
}

// TransferOwnership promotes another member to owner (caller becomes admin).
// Atomic on the server.
func (s *OrgsService) TransferOwnership(ctx context.Context, orgID, newOwnerUserID string) error {
	return s.c.doJSON(ctx, http.MethodPost, "/orgs/"+orgID+"/transfer-ownership", nil,
		TransferOwnershipRequest{UserID: newOwnerUserID}, nil)
}

// ─── invitations ──────────────────────────────────────────────────────────────

func (s *OrgsService) ListInvitations(ctx context.Context, orgID string) ([]Invitation, error) {
	var out []Invitation
	if err := s.c.doJSON(ctx, http.MethodGet, "/orgs/"+orgID+"/invitations", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *OrgsService) Invite(ctx context.Context, orgID string, req InviteRequest) (*Invitation, error) {
	var out Invitation
	if err := s.c.doJSON(ctx, http.MethodPost, "/orgs/"+orgID+"/invitations", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *OrgsService) ResendInvitation(ctx context.Context, orgID, invitationID string) (*Invitation, error) {
	var out Invitation
	if err := s.c.doJSON(ctx, http.MethodPost,
		"/orgs/"+orgID+"/invitations/"+invitationID+"/resend", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *OrgsService) CancelInvitation(ctx context.Context, orgID, invitationID string) error {
	return s.c.doJSON(ctx, http.MethodDelete,
		"/orgs/"+orgID+"/invitations/"+invitationID, nil, nil, nil)
}

// AcceptInvitation joins the org named by the token. The caller must already
// be authenticated as the invitee.
func (s *OrgsService) AcceptInvitation(ctx context.Context, token string) error {
	return s.c.doJSON(ctx, http.MethodPost, "/orgs/invitations/accept", nil,
		AcceptInvitationRequest{Token: token}, nil)
}
