package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.telegram.org"

// api is a minimal Telegram Bot API client using only the stdlib.
type api struct {
	base   string
	token  string
	client *http.Client
	// sleep is overridable in tests for 429 retry handling.
	sleep func(time.Duration)
}

func newAPI(token, baseURL string) *api {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &api{
		base:  strings.TrimRight(baseURL, "/"),
		token: token,
		client: &http.Client{
			Timeout: 65 * time.Second,
		},
		sleep: time.Sleep,
	}
}

type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
	Parameters  *struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

// call POSTs a JSON payload to a Bot API method and decodes the result.
// Never includes the bot token in returned errors.
func (a *api) call(ctx context.Context, method string, payload, result any) error {
	var body []byte
	var err error
	if payload == nil {
		body = []byte("{}")
	} else {
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("telegram %s: marshal payload: %w", method, err)
		}
	}

	url := a.base + "/bot" + a.token + "/" + method
	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("telegram %s: build request: %w", method, err)
		}
		req.Header.Set("Content-Type", "application/json")

		rsp, err := a.client.Do(req)
		if err != nil {
			return fmt.Errorf("telegram %s: %w", method, err)
		}
		raw, readErr := io.ReadAll(rsp.Body)
		rsp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("telegram %s: read body: %w", method, readErr)
		}

		var env apiResponse
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("telegram %s: decode envelope: %w", method, err)
		}
		if !env.OK {
			if env.ErrorCode == 429 && env.Parameters != nil && env.Parameters.RetryAfter > 0 && attempt < maxAttempts {
				a.sleep(time.Duration(env.Parameters.RetryAfter) * time.Second)
				continue
			}
			return fmt.Errorf("telegram %s: %s (code %d)", method, env.Description, env.ErrorCode)
		}
		if result == nil || len(env.Result) == 0 || string(env.Result) == "null" {
			return nil
		}
		if err := json.Unmarshal(env.Result, result); err != nil {
			return fmt.Errorf("telegram %s: decode result: %w", method, err)
		}
		return nil
	}
	return fmt.Errorf("telegram %s: exhausted retries", method)
}

// User is a Telegram user.
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

// Chat is a Telegram chat.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// Message is a Telegram message.
type Message struct {
	MessageID      int64       `json:"message_id"`
	From           *User       `json:"from"`
	Chat           Chat        `json:"chat"`
	Text           string      `json:"text"`
	Photo          []PhotoSize `json:"photo"`
	Document       *Document   `json:"document"`
	Caption        string      `json:"caption"`
	ReplyToMessage *Message    `json:"reply_to_message"`
}

// PhotoSize is a Telegram photo size entry.
type PhotoSize struct {
	FileID   string `json:"file_id"`
	FileSize int64  `json:"file_size"`
}

// Document is a Telegram document.
type Document struct {
	FileID   string `json:"file_id"`
	MimeType string `json:"mime_type"`
	FileName string `json:"file_name"`
}

// CallbackQuery is a Telegram callback query from an inline keyboard.
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

// Update is a Telegram getUpdates entry.
type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

// InlineKeyboardButton is one button on an inline keyboard.
type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// InlineKeyboardMarkup is a Telegram inline keyboard.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

func (a *api) getMe(ctx context.Context) (User, error) {
	var u User
	if err := a.call(ctx, "getMe", map[string]any{}, &u); err != nil {
		return User{}, err
	}
	return u, nil
}

func (a *api) getUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error) {
	var updates []Update
	payload := map[string]any{
		"offset":          offset,
		"timeout":         timeoutSec,
		"allowed_updates": []string{"message", "callback_query"},
	}
	if err := a.call(ctx, "getUpdates", payload, &updates); err != nil {
		return nil, err
	}
	if updates == nil {
		updates = []Update{}
	}
	return updates, nil
}

type sendOpts struct {
	HTML     bool
	Keyboard *InlineKeyboardMarkup
}

func (a *api) sendMessage(ctx context.Context, chatID int64, text string, opts *sendOpts) (Message, error) {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
		"link_preview_options": map[string]any{
			"is_disabled": true,
		},
	}
	if opts != nil {
		if opts.HTML {
			payload["parse_mode"] = "HTML"
		}
		if opts.Keyboard != nil {
			payload["reply_markup"] = opts.Keyboard
		}
	}
	var msg Message
	if err := a.call(ctx, "sendMessage", payload, &msg); err != nil {
		return Message{}, err
	}
	return msg, nil
}

func (a *api) editMessageText(ctx context.Context, chatID, messageID int64, text string, html bool) error {
	payload := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
	}
	if html {
		payload["parse_mode"] = "HTML"
	}
	err := a.call(ctx, "editMessageText", payload, nil)
	if err != nil && strings.Contains(err.Error(), "message is not modified") {
		return nil
	}
	return err
}

func (a *api) answerCallbackQuery(ctx context.Context, id, text string) error {
	payload := map[string]any{
		"callback_query_id": id,
	}
	if text != "" {
		payload["text"] = text
	}
	return a.call(ctx, "answerCallbackQuery", payload, nil)
}

func (a *api) sendChatAction(ctx context.Context, chatID int64, action string) error {
	return a.call(ctx, "sendChatAction", map[string]any{
		"chat_id": chatID,
		"action":  action,
	}, nil)
}
