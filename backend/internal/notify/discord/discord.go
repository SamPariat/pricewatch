// Package discord adapts a discordgo.Session to domain.Notifier.
//
// Unlike the Telegram adapter this replaced, formatting doesn't need any
// escaping discipline (Discord embeds take plain strings, not a markup
// language with reserved characters) — internal/render already produces
// domain.Embed values in adapter-neutral form; this package's only job
// is translating those into discordgo.MessageEmbed and calling the API.
package discord

import (
	"bytes"
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"

	"github.com/SamPariat/pricewatch/internal/domain"
)

// embedsPerMessageLimit is Discord's own cap on embeds in one message —
// CombineDigest can produce more than this for a scheduler group with
// many watches sharing a schedule, so Send batches into multiple
// messages rather than erroring or silently dropping embeds past the
// tenth. Only the first batch carries Content, the image, and buttons —
// same "only the first part" convention the old Telegram adapter used
// when splitting long digests across its own 4096-character limit.
const embedsPerMessageLimit = 10

// Notifier shares its *discordgo.Session with internal/discordbot.Listener
// (which opens the Gateway connection and owns interaction handling) —
// one bot token, one Session, the same relationship the Telegram adapter
// and its long-poll listener had over one bot token.
type Notifier struct {
	session *discordgo.Session
}

func New(session *discordgo.Session) *Notifier {
	return &Notifier{session: session}
}

func (n *Notifier) Send(ctx context.Context, t domain.Target, m domain.Message) error {
	if len(m.Embeds) == 0 {
		send := &discordgo.MessageSend{Content: m.Content, Files: files(m.ImagePNG), Components: components(m.Buttons)}
		if _, err := n.session.ChannelMessageSendComplex(t.ChannelID, send, discordgo.WithContext(ctx)); err != nil {
			return fmt.Errorf("discord: send: %w", err)
		}
		return nil
	}

	for i := 0; i < len(m.Embeds); i += embedsPerMessageLimit {
		end := min(i+embedsPerMessageLimit, len(m.Embeds))
		send := &discordgo.MessageSend{Embeds: toDiscordEmbeds(m.Embeds[i:end])}
		if i == 0 {
			send.Content = m.Content
			send.Files = files(m.ImagePNG)
			send.Components = components(m.Buttons)
		}
		if _, err := n.session.ChannelMessageSendComplex(t.ChannelID, send, discordgo.WithContext(ctx)); err != nil {
			return fmt.Errorf("discord: send batch %d: %w", i/embedsPerMessageLimit+1, err)
		}
	}
	return nil
}

// Status reports whether the Gateway connection is authenticated — a
// cheap in-memory check against Session.State rather than a live API
// call, since internal/discordbot.Listener already keeps that
// connection open for the lifetime of the process. This is strictly
// more accurate than the old Telegram adapter's getMe call, which only
// proved the token parsed, not that the bot was actually reachable.
func (n *Notifier) Status(ctx context.Context) (domain.NotifierStatus, error) {
	if n.session.State == nil || n.session.State.User == nil {
		return domain.NotifierDisconnected, nil
	}
	return domain.NotifierLinked, nil
}

func toDiscordEmbeds(embeds []domain.Embed) []*discordgo.MessageEmbed {
	out := make([]*discordgo.MessageEmbed, len(embeds))
	for i, e := range embeds {
		de := &discordgo.MessageEmbed{
			Title:       e.Title,
			Description: e.Description,
			Color:       e.Color,
		}
		if e.Footer != "" {
			de.Footer = &discordgo.MessageEmbedFooter{Text: e.Footer}
		}
		for _, f := range e.Fields {
			de.Fields = append(de.Fields, &discordgo.MessageEmbedField{Name: f.Name, Value: f.Value, Inline: f.Inline})
		}
		out[i] = de
	}
	return out
}

func components(buttons [][]domain.Button) []discordgo.MessageComponent {
	if len(buttons) == 0 {
		return nil
	}
	rows := make([]discordgo.MessageComponent, len(buttons))
	for i, row := range buttons {
		btns := make([]discordgo.MessageComponent, len(row))
		for j, b := range row {
			btns[j] = discordgo.Button{Label: b.Label, Style: discordgo.SecondaryButton, CustomID: b.Callback}
		}
		rows[i] = discordgo.ActionsRow{Components: btns}
	}
	return rows
}

func files(png []byte) []*discordgo.File {
	if len(png) == 0 {
		return nil
	}
	return []*discordgo.File{{Name: "chart.png", ContentType: "image/png", Reader: bytes.NewReader(png)}}
}
