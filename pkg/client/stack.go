package client

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// StackService wraps the gitops surface from SPEC §2.11.
//
// Types here mirror internal/modules/stack on the wire (same JSON
// shape, same field names). The duplication is deliberate: pkg/client
// is the published SDK, importable by third-party Go users who can't
// reach into internal/, so the contract must live here.
type StackService struct{ c *Client }

// Stack mirrors `internal/modules/stack.Stack`.
type Stack struct {
	APIVersion string        `json:"apiVersion" yaml:"apiVersion"`
	Kind       string        `json:"kind" yaml:"kind"`
	Metadata   StackMetadata `json:"metadata" yaml:"metadata"`
	Spec       StackSpec     `json:"spec" yaml:"spec"`
}

type StackMetadata struct {
	Org string `json:"org,omitempty" yaml:"org,omitempty"`
}

type StackSpec struct {
	Channels []StackChannel `json:"channels,omitempty" yaml:"channels,omitempty"`
	Monitors []StackMonitor `json:"monitors,omitempty" yaml:"monitors,omitempty"`
	Rules    []StackRule    `json:"rules,omitempty" yaml:"rules,omitempty"`
}

type StackChannel struct {
	Name    string         `json:"name" yaml:"name"`
	Type    string         `json:"type" yaml:"type"`
	Enabled *bool          `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Config  map[string]any `json:"config,omitempty" yaml:"config,omitempty"`
}

type StackMonitor struct {
	Name            string            `json:"name" yaml:"name"`
	URL             string            `json:"url" yaml:"url"`
	Method          string            `json:"method,omitempty" yaml:"method,omitempty"`
	HTTPVersion     string            `json:"http_version,omitempty" yaml:"http_version,omitempty"`
	Headers         map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body            string            `json:"body,omitempty" yaml:"body,omitempty"`
	IntervalSeconds int               `json:"interval_seconds,omitempty" yaml:"interval_seconds,omitempty"`
	TimeoutSeconds  int               `json:"timeout_seconds,omitempty" yaml:"timeout_seconds,omitempty"`
	VerifyTLS       *bool             `json:"verify_tls,omitempty" yaml:"verify_tls,omitempty"`
	FollowRedirects *bool             `json:"follow_redirects,omitempty" yaml:"follow_redirects,omitempty"`
	Conditions      []StackCondition  `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

type StackCondition struct {
	Type   string         `json:"type" yaml:"type"`
	Config map[string]any `json:"config,omitempty" yaml:"config,omitempty"`
}

type StackRule struct {
	Name             string            `json:"name" yaml:"name"`
	Trigger          string            `json:"trigger,omitempty" yaml:"trigger,omitempty"`
	FailureThreshold int               `json:"failure_threshold,omitempty" yaml:"failure_threshold,omitempty"`
	RenotifyMinutes  *int              `json:"renotify_minutes,omitempty" yaml:"renotify_minutes,omitempty"`
	AppliesToAll     bool              `json:"applies_to_all,omitempty" yaml:"applies_to_all,omitempty"`
	Monitors         []string          `json:"monitors,omitempty" yaml:"monitors,omitempty"`
	Channels         []string          `json:"channels,omitempty" yaml:"channels,omitempty"`
	Escalations      []StackEscalation `json:"escalations,omitempty" yaml:"escalations,omitempty"`
}

type StackEscalation struct {
	Level        int      `json:"level" yaml:"level"`
	DelayMinutes int      `json:"delay_minutes,omitempty" yaml:"delay_minutes,omitempty"`
	Channels     []string `json:"channels,omitempty" yaml:"channels,omitempty"`
}

// Change / Changeset / ApplyResult mirror the server types verbatim. Note
// that ChangeOp / ChangeKind are plain strings on the wire; we keep them
// as named types client-side so callers can switch on them safely.
type ChangeOp string

const (
	OpCreate ChangeOp = "create"
	OpUpdate ChangeOp = "update"
	OpDelete ChangeOp = "delete"
	OpNoop   ChangeOp = "no-op"
)

type ChangeKind string

const (
	KindChannel ChangeKind = "channel"
	KindMonitor ChangeKind = "monitor"
	KindRule    ChangeKind = "rule"
)

type Change struct {
	Kind  ChangeKind     `json:"kind"`
	Op    ChangeOp       `json:"op"`
	Name  string         `json:"name"`
	Diff  map[string]any `json:"diff,omitempty"`
	Error string         `json:"error,omitempty"`
}

type Changeset struct {
	Changes []Change `json:"changes"`
}

// HasChanges is true when at least one row is not a no-op. Used by the
// CLI for the dry-run exit-code contract (§2.11.4).
func (c *Changeset) HasChanges() bool {
	for _, ch := range c.Changes {
		if ch.Op != OpNoop {
			return true
		}
	}
	return false
}

// ApplyOptions mirrors `internal/modules/stack.ApplyOptions`. Prune is
// nil for the additive default; pass a slice of kinds for scoped
// pruning or `[]ChangeKind{KindChannel, KindMonitor, KindRule}` for all.
type ApplyOptions struct {
	DryRun bool
	Prune  []ChangeKind
}

type ApplyResult struct {
	DryRun    bool       `json:"dry_run"`
	Changeset *Changeset `json:"changeset"`
	FileSHA   string     `json:"file_sha,omitempty"`
}

// Dump retrieves the org's current state as a Stack document.
func (s *StackService) Dump(ctx context.Context, orgID string) (*Stack, error) {
	var out Stack
	if err := s.c.doJSON(ctx, http.MethodGet, "/orgs/"+orgID+"/dump", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Apply reconciles the org to match `stack`. `opts.DryRun = true` makes
// it a server-side diff (no mutations). `opts.Prune` is the comma-CSV
// query param the server expects.
func (s *StackService) Apply(ctx context.Context, orgID string, stack *Stack, opts ApplyOptions) (*ApplyResult, error) {
	q := url.Values{}
	if opts.DryRun {
		q.Set("dry_run", "true")
	}
	if opts.Prune != nil {
		// nil = no prune; empty slice (= prune-all) still wants the
		// query key present with empty value so the server distinguishes
		// it from "no flag at all."
		parts := make([]string, 0, len(opts.Prune))
		for _, k := range opts.Prune {
			parts = append(parts, string(k))
		}
		q.Set("prune", strings.Join(parts, ","))
	}

	var out ApplyResult
	if err := s.c.doJSON(ctx, http.MethodPost, "/orgs/"+orgID+"/apply", q, stack, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
