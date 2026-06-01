package client

import (
	"context"
	"net/http"
)

type DashboardService struct{ c *Client }

type OrgStats struct {
	UpCount      int64 `json:"up_count"`
	DownCount    int64 `json:"down_count"`
	PausedCount  int64 `json:"paused_count"`
	PendingCount int64 `json:"pending_count"`
	TotalCount   int64 `json:"total_count"`
}

type Dashboard struct {
	Stats     *OrgStats  `json:"stats"`
	Incidents []Incident `json:"recent_incidents"`
}

// Get returns the org dashboard: monitor status counts + 10 most recent incidents.
func (s *DashboardService) Get(ctx context.Context, orgID string) (*Dashboard, error) {
	var out Dashboard
	if err := s.c.doJSON(ctx, http.MethodGet, "/orgs/"+orgID+"/dashboard", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
