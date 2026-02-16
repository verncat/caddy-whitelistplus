package whitelistplus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// TelegramBot interacts with the Telegram Bot API to send
// approval requests and process admin callbacks.
type TelegramBot struct {
	token  string
	chatID int64
	store  *Store
	logger *zap.Logger
	client *http.Client
}

// NewTelegramBot creates a new TelegramBot instance.
func NewTelegramBot(token string, chatID int64, store *Store, logger *zap.Logger) *TelegramBot {
	return &TelegramBot{
		token:  token,
		chatID: chatID,
		store:  store,
		logger: logger,
		client: &http.Client{Timeout: 35 * time.Second},
	}
}

func (t *TelegramBot) apiURL(method string) string {
	return fmt.Sprintf("https://api.telegram.org/bot%s/%s", t.token, method)
}

// SendApprovalRequest sends a Telegram message with Approve/Deny
// inline buttons for the given IP.
func (t *TelegramBot) SendApprovalRequest(ip, host, path string) {
	text := fmt.Sprintf(
		"🔒 *WhitelistPlus — New Access Request*\n\n"+
			"*IP:* `%s`\n"+
			"*Host:* `%s`\n"+
			"*Path:* `%s`\n"+
			"*Time:* `%s`",
		ip, host, path,
		time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
	)

	payload := map[string]interface{}{
		"chat_id":    t.chatID,
		"text":       text,
		"parse_mode": "Markdown",
		"reply_markup": map[string]interface{}{
			"inline_keyboard": [][]map[string]string{
				{
					{"text": "✅ Approve", "callback_data": "approve:" + ip},
					{"text": "❌ Deny", "callback_data": "deny:" + ip},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	resp, err := t.client.Post(t.apiURL("sendMessage"), "application/json", bytes.NewReader(body))
	if err != nil {
		t.logger.Error("telegram: failed to send message", zap.Error(err))
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

// Poll starts long-polling the Telegram Bot API for callback
// queries (approve/deny button presses). Blocks until ctx is cancelled.
func (t *TelegramBot) Poll(ctx context.Context) {
	offset := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		url := fmt.Sprintf(
			"%s?offset=%d&timeout=30&allowed_updates=[\"callback_query\"]",
			t.apiURL("getUpdates"), offset,
		)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return
		}

		resp, err := t.client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			t.logger.Error("telegram: poll error", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		var result struct {
			OK     bool `json:"ok"`
			Result []struct {
				UpdateID      int `json:"update_id"`
				CallbackQuery *struct {
					ID      string `json:"id"`
					Data    string `json:"data"`
					Message struct {
						MessageID int `json:"message_id"`
						Chat      struct {
							ID int64 `json:"id"`
						} `json:"chat"`
					} `json:"message"`
				} `json:"callback_query"`
			} `json:"result"`
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err := json.Unmarshal(body, &result); err != nil {
			t.logger.Error("telegram: failed to parse response", zap.Error(err))
			time.Sleep(2 * time.Second)
			continue
		}

		for _, update := range result.Result {
			offset = update.UpdateID + 1

			if update.CallbackQuery == nil {
				continue
			}

			// Only accept callbacks from the configured chat.
			if update.CallbackQuery.Message.Chat.ID != t.chatID {
				continue
			}

			parts := strings.SplitN(update.CallbackQuery.Data, ":", 2)
			if len(parts) != 2 {
				continue
			}

			action, ip := parts[0], parts[1]

			var status, responseText string
			switch action {
			case "approve":
				status = StatusApproved
				responseText = fmt.Sprintf("✅ IP `%s` approved", ip)
			case "deny":
				status = StatusDenied
				responseText = fmt.Sprintf("❌ IP `%s` denied", ip)
			default:
				continue
			}

			if err := t.store.UpdateStatus(ip, status); err != nil {
				t.logger.Error("telegram: failed to update status",
					zap.String("ip", ip), zap.Error(err))
				continue
			}

			t.logger.Info("whitelistplus: IP status updated",
				zap.String("ip", ip),
				zap.String("status", status))

			// Acknowledge the callback and edit the original message.
			t.answerCallback(update.CallbackQuery.ID)
			t.editMessage(
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.MessageID,
				responseText,
			)
		}
	}
}

func (t *TelegramBot) answerCallback(callbackID string) {
	payload, _ := json.Marshal(map[string]string{
		"callback_query_id": callbackID,
	})
	resp, err := t.client.Post(t.apiURL("answerCallbackQuery"), "application/json", bytes.NewReader(payload))
	if err == nil {
		resp.Body.Close()
	}
}

func (t *TelegramBot) editMessage(chatID int64, messageID int, text string) {
	payload, _ := json.Marshal(map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
		"parse_mode": "Markdown",
	})
	resp, err := t.client.Post(t.apiURL("editMessageText"), "application/json", bytes.NewReader(payload))
	if err == nil {
		resp.Body.Close()
	}
}
