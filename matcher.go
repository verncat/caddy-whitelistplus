package whitelistplus

import (
	"fmt"
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(Matcher{})
}

// Matcher is an HTTP request matcher that returns true when the
// client IP is present in the whitelist with status "approved".
//
// Usage in Caddyfile:
//
//	@approved whitelisted
//	handle @approved { ... }
//
// Combine with `not` for the inverse:
//
//	@blocked not whitelisted
type Matcher struct {
	app    *App
	logger *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (Matcher) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.matchers.whitelisted",
		New: func() caddy.Module { return new(Matcher) },
	}
}

// Provision obtains a reference to the shared App module.
func (m *Matcher) Provision(ctx caddy.Context) error {
	m.logger = ctx.Logger()

	appModule, err := ctx.App("whitelistplus")
	if err != nil {
		return fmt.Errorf("whitelistplus app not configured: %v", err)
	}
	m.app = appModule.(*App)

	return nil
}

// Match returns true if the client's IP is approved.
func (m *Matcher) Match(r *http.Request) bool {
	ip := clientAddr(r)

	status, err := m.app.store.GetIPStatus(ip)
	if err != nil {
		m.logger.Error("whitelist matcher: lookup failed", zap.Error(err))
		return false
	}

	return status == StatusApproved
}

// Interface guards
var (
	_ caddy.Module             = (*Matcher)(nil)
	_ caddy.Provisioner        = (*Matcher)(nil)
	_ caddyhttp.RequestMatcher = (*Matcher)(nil)
)
