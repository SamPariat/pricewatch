package domain

// Target identifies where a Notifier sends a Message. For the Telegram
// adapter this is the group chat ID; kept generic so the cloudapi/WhatsApp
// path considered in PLAN.md's history could reuse the same shape.
type Target struct {
	ChatID string
}

// Message is notifier-agnostic: Text is expected to already be formatted
// for the adapter's markup (Telegram HTML parse mode), not raw markdown —
// rendering happens in internal/render, before Notifier.Send is called.
type Message struct {
	Text     string
	ImagePNG []byte // optional chart attachment, nil if none
	Buttons  [][]Button
}

type Button struct {
	Label    string
	Callback string
}
