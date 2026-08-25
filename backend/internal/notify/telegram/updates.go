package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/sampariat/prices-reminder/internal/logging"
)

// Update mirrors the subset of Telegram's Update object this app cares
// about — inline keyboard button presses. Messages, edited messages, and
// every other update type are silently ignored by the caller.
type Update struct {
	UpdateID      int64          `json:"update_id"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

type CallbackQuery struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

type getUpdatesResponse struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

// GetUpdates long-polls for new updates since offset, blocking up to
// timeoutSeconds if none are immediately available — Telegram's own
// recommended pattern, and why this needs its own HTTP client rather
// than Notifier's shared one: that client's fixed 20s timeout would
// abort a 30s long-poll before Telegram ever gets to respond.
func (n *Notifier) GetUpdates(ctx context.Context, offset int64, timeoutSeconds int) ([]Update, error) {
	q := url.Values{}
	q.Set("offset", strconv.FormatInt(offset, 10))
	q.Set("timeout", strconv.Itoa(timeoutSeconds))
	// Only interested in button presses — excluding other update types
	// keeps a chat with regular messages from generating noise this
	// listener would just discard anyway.
	q.Set("allowed_updates", `["callback_query"]`)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.baseURL+n.token+"/getUpdates?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("telegram: build getUpdates request")
	}

	client := http.Client{Timeout: time.Duration(timeoutSeconds+10) * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram: getUpdates request failed")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("telegram: read getUpdates response: %w", err)
	}
	logging.HTTPResponse(ctx, "telegram: getUpdates", resp.StatusCode, time.Since(start), respBody)

	var body getUpdatesResponse
	if err := json.Unmarshal(respBody, &body); err != nil {
		return nil, fmt.Errorf("telegram: decode getUpdates response: %w", err)
	}
	if !body.OK {
		return nil, fmt.Errorf("telegram: getUpdates reported ok=false")
	}
	return body.Result, nil
}

type answerCallbackQueryReq struct {
	CallbackQueryID string `json:"callback_query_id"`
	Text            string `json:"text,omitempty"`
}

// AnswerCallbackQuery clears the loading spinner Telegram shows on a
// pressed inline button and optionally pops a small toast with text —
// this is the confirmation the user sees, e.g. "Snoozed for 7 days."
func (n *Notifier) AnswerCallbackQuery(ctx context.Context, callbackQueryID, text string) error {
	body, err := json.Marshal(answerCallbackQueryReq{CallbackQueryID: callbackQueryID, Text: text})
	if err != nil {
		return err
	}
	return n.call(ctx, "answerCallbackQuery", "application/json", bytes.NewReader(body))
}
