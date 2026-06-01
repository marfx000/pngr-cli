package client

import (
	"context"
	"net/http"
	"time"
)

type RulesService struct{ c *Client }

type Escalation struct {
	ID           string   `json:"id"`
	Level        int      `json:"level"`
	DelayMinutes int      `json:"delay_minutes"`
	ChannelIDs   []string `json:"channel_ids"`
}

type Rule struct {
	ID    string `json:"id"`
	OrgID string `json:"org_id"`
	Name  string `json:"name"`
	// Triggers is a non-empty subset of {on_warning, on_failure,
	// on_recovery} per SPEC §2.8. Multi-select: a rule with multiple
	// triggers fires once per matched trigger per incident.
	Triggers         []string     `json:"triggers"`
	FailureThreshold int          `json:"failure_threshold"`
	RenotifyMinutes  *int         `json:"renotify_minutes"`
	AppliesToAll     bool         `json:"applies_to_all"`
	MonitorIDs       []string     `json:"monitor_ids"`
	ChannelIDs       []string     `json:"channel_ids"`
	Escalations      []Escalation `json:"escalations"`
	CreatedAt        time.Time    `json:"created_at"`
}

type CreateEscalation struct {
	Level        int      `json:"level"`
	DelayMinutes int      `json:"delay_minutes"`
	ChannelIDs   []string `json:"channel_ids"`
}

type CreateRuleRequest struct {
	Name string `json:"name"`
	// Triggers is required (non-empty). Empty/missing array → 400.
	Triggers         []string           `json:"triggers"`
	FailureThreshold int                `json:"failure_threshold,omitempty"`
	RenotifyMinutes  *int               `json:"renotify_minutes,omitempty"`
	MonitorIDs       []string           `json:"monitor_ids,omitempty"`
	ChannelIDs       []string           `json:"channel_ids,omitempty"`
	AppliesToAll     bool               `json:"applies_to_all,omitempty"`
	Escalations      []CreateEscalation `json:"escalations,omitempty"`
}

type UpdateRuleRequest struct {
	Name *string `json:"name,omitempty"`
	// Triggers: nil omits the field (existing array preserved);
	// non-nil replaces. Empty non-nil array → 400.
	Triggers         []string `json:"triggers,omitempty"`
	FailureThreshold *int     `json:"failure_threshold,omitempty"`
	RenotifyMinutes  *int     `json:"renotify_minutes,omitempty"`
	MonitorIDs       []string `json:"monitor_ids,omitempty"`
	ChannelIDs       []string `json:"channel_ids,omitempty"`
}

func (s *RulesService) List(ctx context.Context, orgID string) ([]Rule, error) {
	var out []Rule
	if err := s.c.doJSON(ctx, http.MethodGet, "/orgs/"+orgID+"/rules", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *RulesService) Create(ctx context.Context, orgID string, req CreateRuleRequest) (*Rule, error) {
	var out Rule
	if err := s.c.doJSON(ctx, http.MethodPost, "/orgs/"+orgID+"/rules", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *RulesService) Get(ctx context.Context, orgID, ruleID string) (*Rule, error) {
	var out Rule
	if err := s.c.doJSON(ctx, http.MethodGet,
		"/orgs/"+orgID+"/rules/"+ruleID, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *RulesService) Update(ctx context.Context, orgID, ruleID string, req UpdateRuleRequest) (*Rule, error) {
	var out Rule
	if err := s.c.doJSON(ctx, http.MethodPatch,
		"/orgs/"+orgID+"/rules/"+ruleID, nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *RulesService) Delete(ctx context.Context, orgID, ruleID string) error {
	return s.c.doJSON(ctx, http.MethodDelete,
		"/orgs/"+orgID+"/rules/"+ruleID, nil, nil, nil)
}
