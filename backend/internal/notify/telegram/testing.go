package telegram

// NewForTest builds a Notifier pointed at baseURL (an httptest.Server,
// typically) instead of the real Telegram API. Notifier's fields are all
// unexported, so a genuinely separate test-double package (the storetest
// pattern used elsewhere in this codebase) can't construct one — this is
// the minimal public surface that lets other packages' tests (e.g.
// internal/telegrambot) exercise a fully-functional Notifier without
// hitting the network.
func NewForTest(token, baseURL string) *Notifier {
	n := New(token)
	n.baseURL = baseURL + "/bot"
	return n
}
