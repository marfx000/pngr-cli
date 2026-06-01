// Package client is the typed Go API client for pngr.dev.
//
// Shared by the CLI (cmd/pngr), the MCP server, and third-party Go code that
// wants to integrate with pngr.dev programmatically.
//
//	c := client.New(
//	    client.WithToken(os.Getenv("PNGR_TOKEN")),
//	    client.WithBaseURL("https://api.pngr.dev/v1"),
//	)
//	monitors, err := c.Monitors.List(ctx, orgID)
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.pngr.dev/v1"

// APITokenPrefix brands every personal access token. Exposed so callers can
// sanity-check that a string they're about to send through WithToken is a
// PAT and not an accidental JWT or empty value. Mirrors the server-side
// constant — keep in sync.
const APITokenPrefix = "pngr_pat_"

// Client groups all pngr.dev API operations as sub-resource services. Construct
// it with New(). All sub-services share the same HTTP client, base URL, and
// auth token — there's no per-call allocation cost.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client

	Auth      *AuthService
	Me        *MeService
	Orgs      *OrgsService
	Monitors  *MonitorsService
	Incidents *IncidentsService
	Channels  *ChannelsService
	Rules     *RulesService
	Audit     *AuditService
	Dashboard *DashboardService
	Stack     *StackService
}

type Option func(*Client)

// WithToken sets the bearer token used on every authenticated request.
// Without this, only public endpoints (auth/*) will work.
func WithToken(token string) Option {
	return func(c *Client) { c.token = token }
}

// WithBaseURL overrides the API root. Strip the trailing slash; the client
// adds path separators internally. For local dev this is typically
// "http://localhost:8080/v1".
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient swaps the underlying http.Client. Useful for tests
// (httptest.Server) or to plug in custom transports / proxies.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// New creates a Client with the given options. Sub-service fields are wired
// up here so users can call c.Monitors.List(...) directly.
func New(opts ...Option) *Client {
	c := &Client{
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	c.Auth = &AuthService{c: c}
	c.Me = &MeService{c: c}
	c.Orgs = &OrgsService{c: c}
	c.Monitors = &MonitorsService{c: c}
	c.Incidents = &IncidentsService{c: c}
	c.Channels = &ChannelsService{c: c}
	c.Rules = &RulesService{c: c}
	c.Audit = &AuditService{c: c}
	c.Dashboard = &DashboardService{c: c}
	c.Stack = &StackService{c: c}
	return c
}

// Token returns the currently configured bearer token. Useful for the CLI
// `pngr auth token` command.
func (c *Client) Token() string { return c.token }

// SetToken updates the bearer token in-place. Used after Login() to start
// using the just-issued access token without re-instantiating the client.
func (c *Client) SetToken(token string) { c.token = token }

// ─── error type ───────────────────────────────────────────────────────────────

// APIError is returned for any non-2xx response that carries the standard
// error envelope. Inspect Code for the SCREAMING_SNAKE_CASE error code
// documented in CLAUDE.md / the OpenAPI spec.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("api: status %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("api: %s: %s (status %d)", e.Code, e.Message, e.StatusCode)
}

// IsCode reports whether err is an APIError with the given code. Use to
// branch on specific failure modes:
//
//	if client.IsCode(err, "EMAIL_NOT_VERIFIED") { ... }
func IsCode(err error, code string) bool {
	var apiErr *APIError
	if !errorsAs(err, &apiErr) {
		return false
	}
	return apiErr.Code == code
}

// ─── HTTP plumbing ────────────────────────────────────────────────────────────

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// doJSON marshals body (if non-nil) as JSON, sends the request, and either
// decodes the response body into out (if non-nil) or returns an *APIError.
// query is appended as ?k=v pairs; pass nil for no query params.
func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("client: marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return fmt.Errorf("client: building request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("client: sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("client: reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var env errorEnvelope
		if json.Unmarshal(respBody, &env) == nil && env.Error.Code != "" {
			return &APIError{StatusCode: resp.StatusCode, Code: env.Error.Code, Message: env.Error.Message}
		}
		return &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("client: decoding response: %w", err)
		}
	}
	return nil
}

// errorsAs is a tiny shim so we don't need to import "errors" just for one call
// — keeps this file's dependency surface minimal.
func errorsAs(err error, target any) bool {
	type unwrapper interface{ Unwrap() error }
	for {
		if a, ok := target.(**APIError); ok {
			if v, ok := err.(*APIError); ok {
				*a = v
				return true
			}
		}
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
		if err == nil {
			return false
		}
	}
}
