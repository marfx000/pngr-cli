package client

import (
	"context"
	"net/http"
	"time"
)

type AuditService struct{ c *Client }

type AuditEntry struct {
	ID           string         `json:"id"`
	OrgID        string         `json:"org_id"`
	UserID       *string        `json:"user_id"`
	Action       string         `json:"action"`        // e.g. "monitor.create", "ownership.transferred"
	ResourceType string         `json:"resource_type"` // e.g. "monitor"
	ResourceID   *string        `json:"resource_id"`
	Details      map[string]any `json:"details"`
	CreatedAt    time.Time      `json:"created_at"`
}

// List returns audit entries for an org, newest first. Admin-only on the server.
func (s *AuditService) List(ctx context.Context, orgID string, limit, offset int) ([]AuditEntry, error) {
	var out []AuditEntry
	if err := s.c.doJSON(ctx, http.MethodGet,
		"/orgs/"+orgID+"/audit-log", pagingQuery(limit, offset), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
