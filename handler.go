package whitelistplus

import (
	"fmt"
	"net"
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(Handler{})
}

// Handler is an HTTP middleware that gates access behind an
// IP whitelist. Unknown IPs are registered as pending and an
// approval request is sent to Telegram. Depending on the
// configured action the response is blocked, dropped or a
// placeholder page is shown.
type Handler struct {
	// What to do when the IP is not approved.
	//   block       — return 403 (default)
	//   drop        — silently close the connection
	//   placeholder — return a custom HTML page
	Action string `json:"action,omitempty"`

	// HTML body returned when action is "placeholder".
	Placeholder string `json:"placeholder,omitempty"`

	// HTTP status code for the placeholder response (default 403).
	PlaceholderStatus int `json:"placeholder_status,omitempty"`

	app    *App
	logger *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.whitelistplus",
		New: func() caddy.Module { return new(Handler) },
	}
}

// Provision sets up the handler and obtains a reference to the
// shared App module.
func (h *Handler) Provision(ctx caddy.Context) error {
	h.logger = ctx.Logger()

	appModule, err := ctx.App("whitelistplus")
	if err != nil {
		return fmt.Errorf("whitelistplus app not configured: %v", err)
	}
	h.app = appModule.(*App)

	if h.Action == "" {
		h.Action = "block"
	}
	if h.PlaceholderStatus == 0 {
		h.PlaceholderStatus = http.StatusForbidden
	}
	if h.Placeholder == "" {
		h.Placeholder = defaultPlaceholder
	}

	return nil
}

// Validate checks the handler configuration.
func (h *Handler) Validate() error {
	switch h.Action {
	case "block", "drop", "placeholder":
	default:
		return fmt.Errorf("whitelistplus: invalid action %q (block|drop|placeholder)", h.Action)
	}
	return nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	clientIP := clientAddr(r)

	status, err := h.app.store.GetIPStatus(clientIP)
	if err != nil {
		h.logger.Error("whitelist lookup failed", zap.Error(err))
		// Fail-open: let the request through on DB errors.
		return next.ServeHTTP(w, r)
	}

	// Approved — pass through.
	if status == StatusApproved {
		return next.ServeHTTP(w, r)
	}

	// First time seeing this IP — register it and notify via Telegram.
	if status == StatusUnknown {
		if err := h.app.store.AddIP(clientIP, StatusPending, r.Host, r.URL.Path); err != nil {
			h.logger.Error("failed to register IP", zap.Error(err))
		}

		if h.app.tgBot != nil {
			go h.app.tgBot.SendApprovalRequest(clientIP, r.Host, r.URL.Path)
		}

		h.logger.Info("new IP requesting access",
			zap.String("ip", clientIP),
			zap.String("host", r.Host),
			zap.String("path", r.URL.Path))
	}

	// IP is pending or denied — apply the configured action.
	switch h.Action {
	case "drop":
		hj, ok := w.(http.Hijacker)
		if !ok {
			return caddyhttp.Error(http.StatusForbidden, nil)
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			return caddyhttp.Error(http.StatusForbidden, nil)
		}
		conn.Close()
		return nil

	case "placeholder":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(h.PlaceholderStatus)
		_, _ = w.Write([]byte(h.Placeholder))
		return nil

	default: // block
		return caddyhttp.Error(http.StatusForbidden, nil)
	}
}

// clientAddr extracts the IP address from the request.
func clientAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

const defaultPlaceholder = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Access Pending</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
display:flex;justify-content:center;align-items:center;min-height:100vh;
background:#0f0f0f;color:#e0e0e0}
.c{text-align:center;padding:2rem}
h1{font-size:2rem;margin-bottom:1rem;color:#fff}
p{color:#888;line-height:1.6;max-width:420px}
.icon{font-size:3rem;margin-bottom:1rem}
</style>
</head>
<body>
<div class="c">
<div class="icon">⏳</div>
<h1>Access Pending</h1>
<p>Your IP address has been submitted for approval.
Please wait for an administrator to grant access.</p>
</div>
</body>
</html>`

// Interface guards
var (
	_ caddy.Module                = (*Handler)(nil)
	_ caddy.Provisioner           = (*Handler)(nil)
	_ caddy.Validator             = (*Handler)(nil)
	_ caddyhttp.MiddlewareHandler = (*Handler)(nil)
)
