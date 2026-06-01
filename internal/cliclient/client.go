// Package cliclient wraps client.New with CLI-specific defaults (token + base URL
// from config, sensible error messages when nothing is configured).
package cliclient

import (
	"errors"

	"pngr.dev/cli/internal/cliconfig"
	"pngr.dev/cli/pkg/client"
)

// ErrNotAuthenticated is returned when the config has no token and no
// PNGR_TOKEN env var is set. CLI commands that need auth surface this with a
// "run pngr auth login" hint.
var ErrNotAuthenticated = errors.New("not authenticated; run `pngr auth login` or set PNGR_TOKEN")

// ErrNoActiveOrg is returned when a command needs an org but the config has
// no active_org_id and no --org flag was supplied.
var ErrNoActiveOrg = errors.New("no active organization; run `pngr org use <slug>` or pass --org")

// New builds an authenticated *client.Client from the loaded config. Returns
// ErrNotAuthenticated when no token is available.
func New(cfg *cliconfig.Config) (*client.Client, error) {
	if cfg.Token == "" {
		return nil, ErrNotAuthenticated
	}
	return client.New(
		client.WithBaseURL(cfg.APIURL),
		client.WithToken(cfg.Token),
	), nil
}
