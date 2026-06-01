// Build-tagged smoke test that exercises every client method against a real
// running server. Not run by `go test ./...` — execute with:
//
//	go test -tags=smoke -v -run TestSmoke ./pkg/client
//
// Requires PNGR_BASE_URL (default http://localhost:8080/v1) and a running
// server with email verification configured to log the verify URL (RESEND_API_KEY
// unset on the server side).

//go:build smoke

package client

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSmoke(t *testing.T) {
	baseURL := os.Getenv("PNGR_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080/v1"
	}
	c := New(WithBaseURL(baseURL))
	ctx := context.Background()
	ts := time.Now().UnixNano()

	// Register a fresh user. Server logs the verify URL because RESEND_API_KEY
	// is unset; the operator manually injects the token via PNGR_VERIFY_TOKEN.
	email := strings.ReplaceAll("client-smoke-"+timeStr(ts)+"@example.com", " ", "")
	if _, err := c.Auth.Register(ctx, RegisterRequest{
		Email: email, Password: "supersecure123", DisplayName: "Client Smoke",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	verifyToken := os.Getenv("PNGR_VERIFY_TOKEN")
	if verifyToken == "" {
		t.Skip("set PNGR_VERIFY_TOKEN from server log to run the rest")
	}
	if _, err := c.Auth.VerifyEmail(ctx, verifyToken); err != nil {
		t.Fatalf("verify: %v", err)
	}

	tok, err := c.Auth.Login(ctx, LoginRequest{Email: email, Password: "supersecure123"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	c.SetToken(tok.AccessToken)

	me, err := c.Me.Get(ctx)
	if err != nil || me.Email != email {
		t.Fatalf("get me: %v / email=%s", err, me.Email)
	}

	org, err := c.Orgs.Create(ctx, CreateOrgRequest{Name: "Smoke Test " + timeStr(ts)})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}

	mon, err := c.Monitors.Create(ctx, org.ID, CreateMonitorRequest{
		Name: "Smoke", URL: "https://httpbin.org/status/200",
		IntervalSeconds: 60, TimeoutSeconds: 10,
		Conditions: []CheckCondition{{Type: "status_code", Config: map[string]any{"expected": "200"}}},
	})
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}

	if _, err := c.Monitors.Check(ctx, org.ID, mon.ID); err != nil {
		t.Fatalf("check: %v", err)
	}

	dash, err := c.Dashboard.Get(ctx, org.ID)
	if err != nil || dash.Stats.TotalCount != 1 {
		t.Fatalf("dashboard: %v / total=%d", err, dash.Stats.TotalCount)
	}

	t.Logf("client smoke OK: user=%s org=%s monitor=%s", me.ID, org.ID, mon.ID)
}

func timeStr(ts int64) string {
	const digits = "0123456789"
	out := make([]byte, 0, 20)
	for ts > 0 {
		out = append(out, digits[ts%10])
		ts /= 10
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}
