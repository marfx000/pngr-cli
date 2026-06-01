package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"pngr.dev/cli/internal/cliclient"
	"pngr.dev/cli/pkg/client"
)

// orgHint produces an actionable "no active org / wrong org" message by
// listing the user's actual orgs. Returns "" if the server can't be
// reached or the token doesn't authenticate — in that case the original
// error should be shown instead.
//
// currentOrgID is what the CLI tried to use (or "" if none was configured);
// it's included in the message so the user can correlate with a stale
// `active_org_id` left over from a previous account / db reset.
func orgHint(currentOrgID string) string {
	c, err := cliclient.New(globalConfig)
	if err != nil {
		return ""
	}
	orgs, err := c.Orgs.List(context.Background())
	if err != nil {
		return ""
	}

	var b strings.Builder
	if currentOrgID != "" {
		fmt.Fprintf(&b, "not a member of org %s (stored as active_org_id).\n", currentOrgID)
	}

	switch len(orgs) {
	case 0:
		b.WriteString("you don't belong to any organizations — this shouldn't happen on a normal account; contact support")
	case 1:
		fmt.Fprintf(&b, "you belong to 1 org — run:\n\n      pngr org use %s", orgs[0].Slug)
	default:
		b.WriteString("you belong to multiple orgs:\n\n")
		nameWidth := 0
		for _, o := range orgs {
			if len(o.Name) > nameWidth {
				nameWidth = len(o.Name)
			}
		}
		for _, o := range orgs {
			fmt.Fprintf(&b, "      %-*s  %s\n", nameWidth, o.Name, o.Slug)
		}
		b.WriteString("\n      Run: pngr org use <slug>")
	}
	return b.String()
}

// renderError translates a command error to the user-facing message.
// Centralizes the "stale or missing active_org_id" actionable hint so
// every command benefits without per-call boilerplate.
func renderError(err error) string {
	switch {
	case errors.Is(err, cliclient.ErrNoActiveOrg):
		if hint := orgHint(""); hint != "" {
			return hint
		}
	case client.IsCode(err, "NOT_A_MEMBER"):
		if hint := orgHint(globalConfig.ActiveOrgID); hint != "" {
			return hint
		}
	}
	return err.Error()
}
