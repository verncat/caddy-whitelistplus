package whitelistplus

import (
	"strconv"

	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	httpcaddyfile.RegisterGlobalOption("whitelistplus", parseGlobalOption)
	httpcaddyfile.RegisterHandlerDirective("whitelistplus", parseHandlerDirective)
}

// ---------------------------------------------------------------------------
// Global option: configures the shared App (DB + Telegram).
//
//	{
//	    whitelistplus {
//	        db_path          ./whitelist.db
//	        telegram_token   <token>
//	        telegram_chat_id <chat_id>
//	    }
//	}
//
// ---------------------------------------------------------------------------
func parseGlobalOption(d *caddyfile.Dispenser, existingVal interface{}) (interface{}, error) {
	app := new(App)
	if existingVal != nil {
		var ok bool
		if app, ok = existingVal.(*App); !ok {
			app = new(App)
		}
	}

	for d.Next() {
		for d.NextBlock(0) {
			switch d.Val() {
			case "db_path":
				if !d.NextArg() {
					return nil, d.ArgErr()
				}
				app.DBPath = d.Val()

			case "telegram_token":
				if !d.NextArg() {
					return nil, d.ArgErr()
				}
				app.TelegramToken = d.Val()

			case "telegram_chat_id":
				if !d.NextArg() {
					return nil, d.ArgErr()
				}
				id, err := strconv.ParseInt(d.Val(), 10, 64)
				if err != nil {
					return nil, d.Errf("invalid telegram_chat_id: %v", err)
				}
				app.TelegramChatID = id

			default:
				return nil, d.Errf("unrecognised whitelistplus option: %s", d.Val())
			}
		}
	}

	return httpcaddyfile.App{
		Name:  "whitelistplus",
		Value: caddyconfig.JSON(app, nil),
	}, nil
}

// ---------------------------------------------------------------------------
// Handler directive: configures per-route behaviour.
//
//	whitelistplus {
//	    action             <block|drop|placeholder>
//	    placeholder        "<html>…</html>"
//	    placeholder_status <code>
//	}
//
// Or just `whitelistplus` with no block for defaults (action=block).
// ---------------------------------------------------------------------------
func parseHandlerDirective(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	handler := new(Handler)

	for h.Next() {
		for h.NextBlock(0) {
			switch h.Val() {
			case "action":
				if !h.NextArg() {
					return nil, h.ArgErr()
				}
				handler.Action = h.Val()

			case "placeholder":
				if !h.NextArg() {
					return nil, h.ArgErr()
				}
				handler.Placeholder = h.Val()

			case "placeholder_status":
				if !h.NextArg() {
					return nil, h.ArgErr()
				}
				code, err := strconv.Atoi(h.Val())
				if err != nil {
					return nil, h.Errf("invalid placeholder_status: %v", err)
				}
				handler.PlaceholderStatus = code

			default:
				return nil, h.Errf("unrecognised whitelistplus handler option: %s", h.Val())
			}
		}
	}

	return handler, nil
}

// ---------------------------------------------------------------------------
// Matcher Caddyfile parsing (no arguments).
//
//	@name whitelisted
//
// ---------------------------------------------------------------------------
func (m *Matcher) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		if d.NextArg() {
			return d.ArgErr()
		}
	}
	return nil
}

// Interface guard
var _ caddyfile.Unmarshaler = (*Matcher)(nil)
