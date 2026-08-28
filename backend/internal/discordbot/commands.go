package discordbot

import (
	"github.com/bwmarrin/discordgo"
)

// commands is the full slash-command surface — deliberately minimal:
// add/remove a tracked link, list them, and create/list trips. Nothing
// else, to match exactly what was asked for rather than growing into a
// full trip-management command set.
var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "trip",
		Description: "Manage trips",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "create",
				Description: "Submit a new trip for approval",
				Options: []*discordgo.ApplicationCommandOption{
					{Type: discordgo.ApplicationCommandOptionString, Name: "name", Description: "Trip name", Required: true},
					{Type: discordgo.ApplicationCommandOptionString, Name: "cron", Description: "Cron schedule, e.g. \"0 7 * * *\"", Required: true},
					{Type: discordgo.ApplicationCommandOptionString, Name: "timezone", Description: "IANA timezone, defaults to UTC", Required: false},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "list",
				Description: "List existing trips",
			},
		},
	},
	{
		Name:        "track",
		Description: "Manage tracked links",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "add",
				Description: "Submit an Airbnb link to add to a trip, for approval",
				Options: []*discordgo.ApplicationCommandOption{
					{Type: discordgo.ApplicationCommandOptionString, Name: "trip", Description: "Trip name", Required: true},
					{Type: discordgo.ApplicationCommandOptionString, Name: "url", Description: "Airbnb listing URL", Required: true},
					{Type: discordgo.ApplicationCommandOptionString, Name: "checkin", Description: "Check-in date, YYYY-MM-DD", Required: true},
					{Type: discordgo.ApplicationCommandOptionString, Name: "checkout", Description: "Check-out date, YYYY-MM-DD", Required: true},
					{Type: discordgo.ApplicationCommandOptionInteger, Name: "guests", Description: "Number of guests", Required: false},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "remove",
				Description: "Submit removal of a tracked link, for approval",
				Options: []*discordgo.ApplicationCommandOption{
					{Type: discordgo.ApplicationCommandOptionString, Name: "id", Description: "Watch ID (see /track list)", Required: true},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "list",
				Description: "List tracked Airbnb links",
			},
		},
	},
}

// RegisterCommands overwrites the guild's slash-command set. Guild-scoped
// (not global) so commands appear instantly — matches this app's
// single-server scope; global registration can take up to an hour to
// propagate, a real papercut during iteration.
func RegisterCommands(s *discordgo.Session, appID, guildID string) error {
	_, err := s.ApplicationCommandBulkOverwrite(appID, guildID, commands)
	return err
}
