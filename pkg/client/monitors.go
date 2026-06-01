package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type MonitorsService struct{ c *Client }

type CheckCondition struct {
	ID     string         `json:"id,omitempty"`
	Type   string         `json:"type"` // "status_code" | "latency" | "body_match"
	Config map[string]any `json:"config"`
}

type Monitor struct {
	ID              string            `json:"id"`
	OrgID           string            `json:"org_id"`
	Name            string            `json:"name"`
	URL             string            `json:"url"`
	HTTPVersion     string            `json:"http_version"`
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	IntervalSeconds int               `json:"interval_seconds"`
	TimeoutSeconds  int               `json:"timeout_seconds"`
	// VerifyTLS = false → checker accepts self-signed / invalid certs.
	// HTTPS-only; no-op on http:// monitors.
	VerifyTLS bool `json:"verify_tls"`
	// FollowRedirects = false → checker returns 3xx as-is instead of
	// following the Location chain.
	FollowRedirects bool             `json:"follow_redirects"`
	Status          string           `json:"status"` // "up" | "down" | "paused" | "pending"
	LastCheckedAt   *time.Time       `json:"last_checked_at"`
	NextCheckAt     *time.Time       `json:"next_check_at"`
	Conditions      []CheckCondition `json:"conditions"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	// P95Latency24hMs is p95 over check_results.latency_ms in the last
	// 24h. Zero when no checks landed in the window.
	P95Latency24hMs float64 `json:"p95_24h_ms"`
	// Spark24h is a 12-element latency series for the dashboard
	// sparkline, oldest first, ~2h per bucket, zeros for gaps.
	Spark24h []float64 `json:"spark_24h"`
	// Uptime24hPct is the percentage of check_results in the last 24h
	// where passed=true, in [0, 100]. 100.0 when the monitor has no
	// checks in the window ("no activity yet ≠ 0% uptime").
	Uptime24hPct float64 `json:"uptime_24h_pct"`
}

type CreateMonitorRequest struct {
	Name            string            `json:"name"`
	URL             string            `json:"url"`
	HTTPVersion     string            `json:"http_version,omitempty"`
	Method          string            `json:"method,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	Body            string            `json:"body,omitempty"`
	IntervalSeconds int               `json:"interval_seconds,omitempty"`
	TimeoutSeconds  int               `json:"timeout_seconds,omitempty"`
	// VerifyTLS / FollowRedirects use pointer-bool so callers can
	// distinguish "explicitly false" from "not specified" — the server
	// defaults missing values to true.
	VerifyTLS       *bool            `json:"verify_tls,omitempty"`
	FollowRedirects *bool            `json:"follow_redirects,omitempty"`
	Conditions      []CheckCondition `json:"conditions"`
}

type UpdateMonitorRequest struct {
	Name            *string           `json:"name,omitempty"`
	URL             *string           `json:"url,omitempty"`
	HTTPVersion     *string           `json:"http_version,omitempty"`
	Method          *string           `json:"method,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	Body            *string           `json:"body,omitempty"`
	IntervalSeconds *int              `json:"interval_seconds,omitempty"`
	TimeoutSeconds  *int              `json:"timeout_seconds,omitempty"`
	VerifyTLS       *bool             `json:"verify_tls,omitempty"`
	FollowRedirects *bool             `json:"follow_redirects,omitempty"`
	Conditions      []CheckCondition  `json:"conditions,omitempty"`
}

type CheckResult struct {
	ID             string    `json:"id"`
	MonitorID      string    `json:"monitor_id"`
	CheckedAt      time.Time `json:"checked_at"`
	StatusCode     *int32    `json:"status_code"`
	LatencyMs      *int64    `json:"latency_ms"`
	BodyExcerpt    string    `json:"body_excerpt"`
	Passed         bool      `json:"passed"`
	FailureReasons []string  `json:"failure_reasons"`
}

type UptimeStats struct {
	UptimePct   float64 `json:"uptime_pct"`
	SampleCount int64   `json:"sample_count"`
}

type ResponseTimeBucket struct {
	Bucket       time.Time `json:"bucket"`
	AvgLatencyMs float64   `json:"avg_latency_ms"`
	MaxLatencyMs int64     `json:"max_latency_ms"`
	MinLatencyMs int64     `json:"min_latency_ms"`
	SampleCount  int64     `json:"sample_count"`
	SuccessCount int64     `json:"success_count"`
}

// ─── CRUD ─────────────────────────────────────────────────────────────────────

func (s *MonitorsService) List(ctx context.Context, orgID string) ([]Monitor, error) {
	var out []Monitor
	if err := s.c.doJSON(ctx, http.MethodGet, "/orgs/"+orgID+"/monitors", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *MonitorsService) Create(ctx context.Context, orgID string, req CreateMonitorRequest) (*Monitor, error) {
	var out Monitor
	if err := s.c.doJSON(ctx, http.MethodPost, "/orgs/"+orgID+"/monitors", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *MonitorsService) Get(ctx context.Context, orgID, monitorID string) (*Monitor, error) {
	var out Monitor
	if err := s.c.doJSON(ctx, http.MethodGet,
		"/orgs/"+orgID+"/monitors/"+monitorID, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *MonitorsService) Update(ctx context.Context, orgID, monitorID string, req UpdateMonitorRequest) (*Monitor, error) {
	var out Monitor
	if err := s.c.doJSON(ctx, http.MethodPatch,
		"/orgs/"+orgID+"/monitors/"+monitorID, nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *MonitorsService) Delete(ctx context.Context, orgID, monitorID string) error {
	return s.c.doJSON(ctx, http.MethodDelete,
		"/orgs/"+orgID+"/monitors/"+monitorID, nil, nil, nil)
}

func (s *MonitorsService) Pause(ctx context.Context, orgID, monitorID string) error {
	return s.c.doJSON(ctx, http.MethodPost,
		"/orgs/"+orgID+"/monitors/"+monitorID+"/pause", nil, nil, nil)
}

func (s *MonitorsService) Resume(ctx context.Context, orgID, monitorID string) error {
	return s.c.doJSON(ctx, http.MethodPost,
		"/orgs/"+orgID+"/monitors/"+monitorID+"/resume", nil, nil, nil)
}

// Check fires a synchronous check and returns the result. The result is also
// persisted server-side asynchronously, so a follow-up ListResults will see it.
func (s *MonitorsService) Check(ctx context.Context, orgID, monitorID string) (*CheckResult, error) {
	var out CheckResult
	if err := s.c.doJSON(ctx, http.MethodPost,
		"/orgs/"+orgID+"/monitors/"+monitorID+"/check", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── time series ──────────────────────────────────────────────────────────────

func (s *MonitorsService) ListResults(ctx context.Context, orgID, monitorID string, limit, offset int) ([]CheckResult, error) {
	q := pagingQuery(limit, offset)
	var out []CheckResult
	if err := s.c.doJSON(ctx, http.MethodGet,
		"/orgs/"+orgID+"/monitors/"+monitorID+"/results", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Uptime returns the % uptime over the requested range ("24h" | "7d" | "30d").
func (s *MonitorsService) Uptime(ctx context.Context, orgID, monitorID, rangeStr string) (*UptimeStats, error) {
	q := url.Values{}
	if rangeStr != "" {
		q.Set("range", rangeStr)
	}
	var out UptimeStats
	if err := s.c.doJSON(ctx, http.MethodGet,
		"/orgs/"+orgID+"/monitors/"+monitorID+"/uptime", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ResponseTime returns latency stats bucketed by hour (24h) or day (7d/30d).
func (s *MonitorsService) ResponseTime(ctx context.Context, orgID, monitorID, rangeStr string) ([]ResponseTimeBucket, error) {
	q := url.Values{}
	if rangeStr != "" {
		q.Set("range", rangeStr)
	}
	var out []ResponseTimeBucket
	if err := s.c.doJSON(ctx, http.MethodGet,
		"/orgs/"+orgID+"/monitors/"+monitorID+"/response-time", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func pagingQuery(limit, offset int) url.Values {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	return q
}
