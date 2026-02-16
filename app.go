package whitelistplus

import (
	"context"
	"os"
	"strconv"
	"strings"

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

	// Telegram chat ID where approval requests are sent (as string to support env vars).
	TelegramChatIDRaw string `json:"telegram_chat_id_raw,omitempty"`

	// Telegram chat ID where approval requests are sent.
	TelegramChatID int64 `json:"telegram_chat_id,omitempty"`

	// Custom message template for Telegram notifications.
	// Supports {{.IP}}, {{.Host}}, {{.Path}}, {{.Time}}
	TelegramMessage string `json:"telegram_message,omitempty"`

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

	// Parse TelegramChatIDRaw (which may contain env vars) into TelegramChatID
	if a.TelegramChatIDRaw != "" {
		// Expand environment variables (e.g., {env.TG_CHAT_ID})
		chatIDStr := expandEnvVars(a.TelegramChatIDRaw)

		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			a.logger.Error("failed to parse telegram_chat_id",
				zap.String("raw", a.TelegramChatIDRaw),
				zap.String("expanded", chatIDStr),
				zap.Error(err))
			return err
		}
		a.TelegramChatID = chatID
		a.logger.Info("telegram chat ID configured", zap.Int64("chat_id", chatID))
	}

	// Expand env vars in telegram token too
	if a.TelegramToken != "" {
		a.TelegramToken = expandEnvVars(a.TelegramToken)
	}

	store, err := NewStore(a.DBPath)
	if err != nil {
		return err
	}
	a.store = store

	if a.TelegramToken != "" && a.TelegramChatID != 0 {
		a.tgBot = NewTelegramBot(a.TelegramToken, a.TelegramChatID, a.TelegramMessage, a.store, a.logger)
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

// expandEnvVars expands environment variables in the Caddyfile format {env.VAR}
func expandEnvVars(s string) string {
	// Replace {env.VAR_NAME} with the value of VAR_NAME
	if !strings.Contains(s, "{env.") {
		return s
	}

	// Simple replacement for {env.VAR} format
	result := s
	start := 0
	for {
		idx := strings.Index(result[start:], "{env.")
		if idx == -1 {
			break
		}
		idx += start

		endIdx := strings.Index(result[idx:], "}")
		if endIdx == -1 {
			break
		}
		endIdx += idx

		// Extract variable name
		varName := result[idx+5 : endIdx] // +5 to skip "{env."
		varValue := os.Getenv(varName)

		// Replace {env.VAR} with value
		result = result[:idx] + varValue + result[endIdx+1:]
		start = idx + len(varValue)
	}

	return result
}

// Interface guards
var (
	_ caddy.App         = (*App)(nil)
	_ caddy.Module      = (*App)(nil)
	_ caddy.Provisioner = (*App)(nil)
)
