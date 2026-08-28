package discord

import (
	"context"
	"testing"

	"github.com/bwmarrin/discordgo"

	"github.com/SamPariat/pricewatch/internal/domain"
)

func TestToDiscordEmbeds_MapsFieldsAndFooter(t *testing.T) {
	in := []domain.Embed{
		{
			Title: "BLR → GOI", Description: "**INR 8,412**", Color: 0x22c55e,
			Fields: []domain.EmbedField{{Name: "Change", Value: "▼ 4.6%", Inline: true}},
			Footer: "No exact match",
		},
	}
	out := toDiscordEmbeds(in)
	if len(out) != 1 {
		t.Fatalf("got %d embeds, want 1", len(out))
	}
	e := out[0]
	if e.Title != "BLR → GOI" || e.Description != "**INR 8,412**" || e.Color != 0x22c55e {
		t.Errorf("embed = %+v, want the input fields carried through unchanged", e)
	}
	if len(e.Fields) != 1 || e.Fields[0].Name != "Change" || e.Fields[0].Value != "▼ 4.6%" || !e.Fields[0].Inline {
		t.Errorf("Fields = %+v, want one Change field carried through", e.Fields)
	}
	if e.Footer == nil || e.Footer.Text != "No exact match" {
		t.Errorf("Footer = %+v, want Text %q", e.Footer, "No exact match")
	}
}

func TestToDiscordEmbeds_NoFooter_LeavesFooterNil(t *testing.T) {
	out := toDiscordEmbeds([]domain.Embed{{Title: "a"}})
	if out[0].Footer != nil {
		t.Errorf("Footer = %+v, want nil when the domain embed has no footer", out[0].Footer)
	}
}

func TestComponents_EmptyButtons_ReturnsNil(t *testing.T) {
	if got := components(nil); got != nil {
		t.Errorf("components(nil) = %+v, want nil", got)
	}
	if got := components([][]domain.Button{}); got != nil {
		t.Errorf("components([]) = %+v, want nil", got)
	}
}

func TestComponents_BuildsActionRowsWithCustomID(t *testing.T) {
	buttons := [][]domain.Button{
		{{Label: "Snooze 7d", Callback: "snooze:w1"}, {Label: "Pause", Callback: "pause:w1"}},
		{{Label: "Refresh now", Callback: "refresh:w1"}},
	}
	rows := components(buttons)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	row0, ok := rows[0].(discordgo.ActionsRow)
	if !ok {
		t.Fatalf("rows[0] = %T, want discordgo.ActionsRow", rows[0])
	}
	if len(row0.Components) != 2 {
		t.Fatalf("row0 has %d components, want 2", len(row0.Components))
	}
	btn, ok := row0.Components[0].(discordgo.Button)
	if !ok {
		t.Fatalf("row0.Components[0] = %T, want discordgo.Button", row0.Components[0])
	}
	if btn.Label != "Snooze 7d" || btn.CustomID != "snooze:w1" {
		t.Errorf("button = %+v, want Label %q CustomID %q", btn, "Snooze 7d", "snooze:w1")
	}
}

func TestFiles_EmptyPNG_ReturnsNil(t *testing.T) {
	if got := files(nil); got != nil {
		t.Errorf("files(nil) = %+v, want nil", got)
	}
	if got := files([]byte{}); got != nil {
		t.Errorf("files([]byte{}) = %+v, want nil", got)
	}
}

func TestFiles_NonEmptyPNG_ReturnsOneFile(t *testing.T) {
	got := files([]byte{0x89, 0x50, 0x4e, 0x47})
	if len(got) != 1 {
		t.Fatalf("got %d files, want 1", len(got))
	}
	if got[0].Name != "chart.png" || got[0].ContentType != "image/png" {
		t.Errorf("file = %+v, want chart.png/image/png", got[0])
	}
}

// TestStatus_UnauthenticatedSession_ReportsDisconnected is the only
// Status behavior testable without a live Gateway connection: a
// freshly-constructed Session has never received a Ready event, so
// State.User is nil.
func TestStatus_UnauthenticatedSession_ReportsDisconnected(t *testing.T) {
	session, err := discordgo.New("Bot faketoken")
	if err != nil {
		t.Fatal(err)
	}
	n := New(session)
	status, err := n.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status != domain.NotifierDisconnected {
		t.Errorf("Status = %q, want disconnected for a session that was never opened", status)
	}
}
