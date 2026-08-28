package domain

// Target identifies where a Notifier sends a Message — for the Discord
// adapter this is a channel ID.
type Target struct {
	ChannelID string
}

// Message is notifier-agnostic: Embeds carry the actual rendered digest
// content (see internal/render), which the Discord adapter translates
// into discordgo.MessageEmbed — internal/render deliberately doesn't
// import discordgo itself, so the domain core stays free of any one
// adapter's SDK types (PLAN.md § Architecture patterns: only the
// adapters know about a specific external API's shapes).
type Message struct {
	// Content is optional plain text alongside the embed(s) — e.g. the
	// "Daily digest — N watches" header CombineDigest builds, or the
	// "🔔 Threshold alert" prefix a breached watch's alert gets.
	Content  string
	Embeds   []Embed
	ImagePNG []byte // optional chart attachment, nil if none
	Buttons  [][]Button
}

// Embed is a Discord embed's contents in adapter-agnostic form — the
// same "rich card" idea Telegram doesn't have a native equivalent for,
// which is part of why the port grew this shape when the notifier
// switched from Telegram to Discord.
type Embed struct {
	Title       string
	Description string
	Color       int // 0xRRGGBB
	Fields      []EmbedField
	Footer      string
}

type EmbedField struct {
	Name   string
	Value  string
	Inline bool
}

type Button struct {
	Label    string
	Callback string // Discord message-component custom_id, "action:watchID"
}
