package client

import (
	"context"
	"net/http"
	"time"
)

type IncidentsService struct{ c *Client }

type Incident struct {
	ID              string     `json:"id"`
	MonitorID       string     `json:"monitor_id"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at"`
	DurationSeconds *int64     `json:"duration_seconds"`
	Resolved        bool       `json:"resolved"`
	// Severity is the *current* severity — "warning" or "critical".
	// Flips in place between degraded↔down without opening a new incident.
	Severity string `json:"severity"`
	// HighWaterSeverity preserves the worst severity the incident reached.
	HighWaterSeverity string `json:"high_water_severity"`
}

// ListByOrg returns recent incidents across all monitors in the org.
func (s *IncidentsService) ListByOrg(ctx context.Context, orgID string, limit, offset int) ([]Incident, error) {
	var out []Incident
	if err := s.c.doJSON(ctx, http.MethodGet,
		"/orgs/"+orgID+"/incidents", pagingQuery(limit, offset), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListByMonitor returns incidents for a specific monitor.
func (s *IncidentsService) ListByMonitor(ctx context.Context, orgID, monitorID string, limit, offset int) ([]Incident, error) {
	var out []Incident
	if err := s.c.doJSON(ctx, http.MethodGet,
		"/orgs/"+orgID+"/monitors/"+monitorID+"/incidents", pagingQuery(limit, offset), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get returns a single incident scoped to the org. Returns a 404 APIError if
// the incident belongs to a different org, even when the UUID is valid.
func (s *IncidentsService) Get(ctx context.Context, orgID, incidentID string) (*Incident, error) {
	var out Incident
	if err := s.c.doJSON(ctx, http.MethodGet,
		"/orgs/"+orgID+"/incidents/"+incidentID, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
