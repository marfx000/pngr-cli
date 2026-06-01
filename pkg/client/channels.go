package client

import (
	"context"
	"net/http"
	"time"
)

type ChannelsService struct{ c *Client }

type Channel struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // "email" | "slack" | "ms_teams" | "telegram" | "webhook" | "sms" | "pagerduty" | "opsgenie"
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	// Config is intentionally omitted from API responses — credentials are
	// write-only. Construct it for Create/Update calls only.
}

// CreateChannelRequest config shape varies by Type:
//   - slack / ms_teams: {"webhook_url": "..."}
//   - telegram:         {"bot_token": "...", "chat_id": "..."}
//   - email:            {"address": "..."}
//   - webhook:          {"url": "..."}
//   - sms:              {"to": "+15551234567"}  (provider configured server-side)
//   - pagerduty:        {"routing_key": "..."}  (Events API v2; trigger/resolve)
//   - opsgenie:         {"api_key": "...", "region": "us"|"eu"}  (open/close)
type CreateChannelRequest struct {
	Name   string         `json:"name"`
	Type   string         `json:"type"`
	Config map[string]any `json:"config"`
}

type UpdateChannelRequest struct {
	Name    *string        `json:"name,omitempty"`
	Config  map[string]any `json:"config,omitempty"`  // nil leaves config unchanged
	Enabled *bool          `json:"enabled,omitempty"` // nil leaves enabled flag unchanged
}

func (s *ChannelsService) List(ctx context.Context, orgID string) ([]Channel, error) {
	var out []Channel
	if err := s.c.doJSON(ctx, http.MethodGet, "/orgs/"+orgID+"/channels", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *ChannelsService) Create(ctx context.Context, orgID string, req CreateChannelRequest) (*Channel, error) {
	var out Channel
	if err := s.c.doJSON(ctx, http.MethodPost, "/orgs/"+orgID+"/channels", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *ChannelsService) Update(ctx context.Context, orgID, channelID string, req UpdateChannelRequest) (*Channel, error) {
	var out Channel
	if err := s.c.doJSON(ctx, http.MethodPatch,
		"/orgs/"+orgID+"/channels/"+channelID, nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *ChannelsService) Delete(ctx context.Context, orgID, channelID string) error {
	return s.c.doJSON(ctx, http.MethodDelete,
		"/orgs/"+orgID+"/channels/"+channelID, nil, nil, nil)
}

// Test sends a sample notification through the channel. Returns nil on
// successful dispatch, an APIError with status 502 if the underlying
// webhook / SMTP call fails.
func (s *ChannelsService) Test(ctx context.Context, orgID, channelID string) error {
	return s.c.doJSON(ctx, http.MethodPost,
		"/orgs/"+orgID+"/channels/"+channelID+"/test", nil, nil, nil)
}
