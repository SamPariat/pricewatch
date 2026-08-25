// Package telegram adapts the Telegram Bot API to domain.Notifier.
//
// Formatting uses HTML parse mode, not MarkdownV2 — MarkdownV2 requires
// escaping ~18 reserved characters and is a reliable source of runtime
// formatting bugs (PLAN.md § Traps). A chart attaches via sendPhoto when
// Message.ImagePNG is set. Messages over Telegram's 4096-character limit
// split on the "\n\n" section boundaries render.CombineDigest already
// produces, sent as separate calls — only the first carries the image and
// buttons.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/sampariat/prices-reminder/internal/domain"
	"github.com/sampariat/prices-reminder/internal/logging"
)

const (
	defaultBaseURL = "https://api.telegram.org/bot"
	messageLimit   = 4096
)

type Notifier struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

func New(token string) *Notifier {
	return &Notifier{
		token:      token,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
}

type inlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

type replyMarkup struct {
	InlineKeyboard [][]inlineButton `json:"inline_keyboard"`
}

func toReplyMarkup(buttons [][]domain.Button) *replyMarkup {
	if len(buttons) == 0 {
		return nil
	}
	rows := make([][]inlineButton, len(buttons))
	for i, row := range buttons {
		btnRow := make([]inlineButton, len(row))
		for j, b := range row {
			btnRow[j] = inlineButton{Text: b.Label, CallbackData: b.Callback}
		}
		rows[i] = btnRow
	}
	return &replyMarkup{InlineKeyboard: rows}
}

// Send delivers a digest, splitting long text across multiple messages
// and attaching Message.ImagePNG (if present) and Buttons to only the
// first one.
func (n *Notifier) Send(ctx context.Context, t domain.Target, m domain.Message) error {
	parts := splitMessage(m.Text, messageLimit)
	if len(parts) == 0 {
		parts = []string{""}
	}

	for i, part := range parts {
		var img []byte
		var markup *replyMarkup
		if i == 0 {
			img = m.ImagePNG
			markup = toReplyMarkup(m.Buttons)
		}
		var err error
		if len(img) > 0 {
			err = n.sendPhoto(ctx, t, part, img, markup)
		} else {
			err = n.sendMessage(ctx, t, part, markup)
		}
		if err != nil {
			return fmt.Errorf("telegram: send part %d/%d: %w", i+1, len(parts), err)
		}
	}
	return nil
}

type sendMessageReq struct {
	ChatID      string       `json:"chat_id"`
	Text        string       `json:"text"`
	ParseMode   string       `json:"parse_mode"`
	ReplyMarkup *replyMarkup `json:"reply_markup,omitempty"`
}

func (n *Notifier) sendMessage(ctx context.Context, t domain.Target, text string, markup *replyMarkup) error {
	body, err := json.Marshal(sendMessageReq{ChatID: t.ChatID, Text: text, ParseMode: "HTML", ReplyMarkup: markup})
	if err != nil {
		return err
	}
	return n.call(ctx, "sendMessage", "application/json", bytes.NewReader(body))
}

func (n *Notifier) sendPhoto(ctx context.Context, t domain.Target, caption string, png []byte, markup *replyMarkup) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if err := w.WriteField("chat_id", t.ChatID); err != nil {
		return err
	}
	if caption != "" {
		if err := w.WriteField("caption", caption); err != nil {
			return err
		}
		if err := w.WriteField("parse_mode", "HTML"); err != nil {
			return err
		}
	}
	if markup != nil {
		mj, err := json.Marshal(markup)
		if err != nil {
			return err
		}
		if err := w.WriteField("reply_markup", string(mj)); err != nil {
			return err
		}
	}
	fw, err := w.CreateFormFile("photo", "chart.png")
	if err != nil {
		return err
	}
	if _, err := fw.Write(png); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	return n.call(ctx, "sendPhoto", w.FormDataContentType(), &buf)
}

// Status calls getMe to confirm the bot token is valid and Telegram is
// reachable — the panel's channel-status indicator reads this.
func (n *Notifier) Status(ctx context.Context) (domain.NotifierStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.baseURL+n.token+"/getMe", nil)
	if err != nil {
		// A request-construction error could embed the URL (and so the
		// bot token) in its message — never propagate it raw, matching
		// the same discipline applied in call() below.
		return domain.NotifierDisconnected, fmt.Errorf("telegram: build status request")
	}
	start := time.Now()
	resp, err := n.httpClient.Do(req)
	if err != nil {
		return domain.NotifierDisconnected, nil // network trouble is "disconnected," not a caller error
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.NotifierDisconnected, nil
	}
	logging.HTTPResponse(ctx, "telegram: getMe", resp.StatusCode, time.Since(start), respBody)

	var body apiResponse
	if err := json.Unmarshal(respBody, &body); err != nil || !body.OK {
		return domain.NotifierDisconnected, nil
	}
	return domain.NotifierLinked, nil
}

func (n *Notifier) call(ctx context.Context, method, contentType string, body io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.baseURL+n.token+"/"+method, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	start := time.Now()
	resp, err := n.httpClient.Do(req)
	if err != nil {
		// The bot token is part of the URL path — never let this error,
		// or any logging of it, include the request URL.
		return fmt.Errorf("request failed")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	logging.HTTPResponse(ctx, "telegram: "+method, resp.StatusCode, time.Since(start), respBody)

	var out apiResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !out.OK {
		return fmt.Errorf("telegram api error: %s", out.Description)
	}
	return nil
}

// splitMessage packs "\n\n"-separated sections into chunks no larger than
// limit, splitting between sections rather than mid-word. A single
// section longer than limit is hard-truncated — pathological, but must
// not hang or panic.
func splitMessage(text string, limit int) []string {
	if text == "" {
		return nil
	}
	sections := strings.Split(text, "\n\n")

	var parts []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			parts = append(parts, current.String())
			current.Reset()
		}
	}

	for _, sec := range sections {
		if len(sec) > limit {
			sec = sec[:limit]
		}
		candidateLen := current.Len() + len(sec)
		if current.Len() > 0 {
			candidateLen += 2 // the "\n\n" that would join them
		}
		if candidateLen > limit {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(sec)
	}
	flush()
	return parts
}
