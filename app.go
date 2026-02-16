package whitelistplus

import (
	"context"

	"github.com/caddyserver/caddy/v2"
	"go.uber.org/zap"
)

func init() {
	caddy.RegisterModule(App{})
}

// App is the top-level Caddy app module that holds shared state:
// SQLite store and Telegram bot instance. Configured via the
// global `whitelistplus` option in Caddyfile.
type App struct {
	// Path to the SQLite database file.
	DBPath string `json:"db_path,omitempty"`

	// Telegram Bot API token.
	TelegramToken string `json:"telegram_token,omitempty"`

	// Telegram chat ID where approval requests are sent.
	TelegramChatID int64 `json:"telegram_chat_id,omitempty"`

	store  *Store
	tgBot  *TelegramBot
	cancel context.CancelFunc
	logger *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (App) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "whitelistplus",
		New: func() caddy.Module { return new(App) },
	}
}

// Provision sets up the app: opens the SQLite database and
// initialises the Telegram bot (if configured).
func (a *App) Provision(ctx caddy.Context) error {
	a.logger = ctx.Logger()

	if a.DBPath == "" {
		a.DBPath = "whitelist.db"
	}

	store, err := NewStore(a.DBPath)
	if err != nil {
		return err
	}
	a.store = store

	if a.TelegramToken != "" && a.TelegramChatID != 0 {
		a.tgBot = NewTelegramBot(a.TelegramToken, a.TelegramChatID, a.store, a.logger)
	}

	return nil
}

// Start begins the Telegram polling goroutine.
func (a *App) Start() error {
	if a.tgBot != nil {
		ctx, cancel := context.WithCancel(context.Background())
		a.cancel = cancel
		go a.tgBot.Poll(ctx)
		a.logger.Info("whitelistplus: telegram polling started")
	}
	return nil
}

// Stop cleans up resources.
func (a *App) Stop() error {
	if a.cancel != nil {
		a.cancel()
	}
	if a.store != nil {
		a.store.Close()
	}
	a.logger.Info("whitelistplus: stopped")
	return nil
}

// Interface guards
var (
	_ caddy.App         = (*App)(nil)
	_ caddy.Module      = (*App)(nil)
	_ caddy.Provisioner = (*App)(nil)
)
