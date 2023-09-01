package bot

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/logger"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/api/cmdroute"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/diamondburned/arikawa/v3/utils/sendpart"
	"github.com/jellydator/ttlcache/v3"
	"github.com/pkg/errors"
	"libdb.so/xcsv"
	"tailscale.com/util/singleflight"
)

var globalCommands = []api.CreateCommandData{
	{
		Name:        "init-register",
		Description: "initialize a channel to post a message",
		Options: []discord.CommandOption{
			&discord.ChannelOption{
				OptionName:  "channel",
				Description: "the channel to make a message in",
				Required:    true,
				ChannelTypes: []discord.ChannelType{
					discord.GuildText,
				},
			},
			&discord.RoleOption{
				OptionName:  "registered-role",
				Description: "the role to give when the user is registered",
				Required:    true,
			},
			&discord.StringOption{
				OptionName:  "message",
				Description: "body for the message",
				Required:    true,
			},
			&discord.StringOption{
				OptionName:  "register-button-label",
				Description: "the text for the Register button, default 'Register'",
			},
			&discord.StringOption{
				OptionName:  "registered-message",
				Description: "the message to reply once registered successfully, default \"You're all set!\"",
			},
		},
	},
	{
		Name:        "registered-member",
		Description: "group of commands that are member-related specific to this guild",
		Options: []discord.CommandOption{
			&discord.SubcommandOption{
				OptionName:  "query",
				Description: "query the info of a known user or error if not registered",
				Options: []discord.CommandOptionValue{
					&discord.UserOption{
						OptionName:  "who",
						Description: "the user to query",
						Required:    true,
					},
				},
			},
			&discord.SubcommandOption{
				OptionName:  "unregister",
				Description: "unregister a user and remove their role",
				Options: []discord.CommandOptionValue{
					&discord.UserOption{
						OptionName:  "who",
						Description: "the user to unregister",
						Required:    true,
					},
				},
			},
			&discord.SubcommandOption{
				OptionName:  "reset-name",
				Description: "change the name of a registered member back to the default one",
				Options: []discord.CommandOptionValue{
					&discord.UserOption{
						OptionName:  "who",
						Description: "the user to rename",
						Required:    true,
					},
				},
			},
		},
	},
	{
		Name:        "event-registration",
		Description: "commands for relating Discord events to the registration database",
		Options: []discord.CommandOption{
			&discord.SubcommandOption{
				OptionName:  "export-members",
				Description: "export all members participating in an event to a CSV file",
				Options: []discord.CommandOptionValue{
					&discord.StringOption{
						OptionName:   "event",
						Description:  "the event to export the participants of",
						Required:     true,
						Autocomplete: true,
					},
				},
			},
		},
	},
	{
		Name:        "clear-registration",
		Description: "clear the Register message",
	},
}

// OverwriteCommands overwrites the commands to the ones defined in Commands.
func (h *Handler) OverwriteCommands() error {
	app, err := h.s.CurrentApplication()
	if err != nil {
		return errors.Wrap(err, "cannot get current app")
	}

	_, err = h.s.BulkOverwriteCommands(app.ID, globalCommands)
	if err != nil {
		return errors.Wrap(err, "cannot overwrite old commands")
	}

	return nil
}

func (h *Handler) cmdInitRegister(ctx context.Context, cmdData cmdroute.CommandData) *api.InteractionResponseData {
	_, err := h.store.GuildInfo(cmdData.Event.GuildID)
	if err == nil {
		return ErrorResponseData(errors.New("guild is already registered; clear it first"))
	}

	var data struct {
		ChannelID             discord.ChannelID `discord:"channel"`
		RegisteredRole        discord.RoleID    `discord:"registered-role"`
		Message               string            `discord:"message"`
		RegisteredButtonLabel string            `discord:"register-button-label?"`
		RegisteredMessage     string            `discord:"registered-message?"`
	}

	if err := cmdData.Options.Unmarshal(&data); err != nil {
		return ErrorResponseData(err)
	}

	if data.RegisteredButtonLabel == "" {
		data.RegisteredButtonLabel = registeredButtonLabel
	}

	if data.RegisteredMessage == "" {
		data.RegisteredMessage = registeredMessage
	}

	registerMsg, err := h.s.SendMessageComplex(data.ChannelID, api.SendMessageData{
		Content: data.Message,
		Components: []discord.ContainerComponent{
			&discord.ActionRowComponent{
				&discord.ButtonComponent{
					Style:    discord.PrimaryButtonStyle(),
					CustomID: "register",
					Label:    data.RegisteredButtonLabel,
				},
			},
		},
	})
	if err != nil {
		return ErrorResponseData(errors.Wrap(err, "cannot send register message"))
	}

	if err := h.store.InitGuild(acmregister.KnownGuild{
		GuildID:           cmdData.Event.GuildID,
		ChannelID:         data.ChannelID,
		RoleID:            data.RegisteredRole,
		InitUserID:        cmdData.Event.SenderID(),
		RegisteredMessage: data.RegisteredMessage,
	}); err != nil {
		h.s.DeleteMessage(data.ChannelID, registerMsg.ID, "cannot init guild, check error")
		return ErrorResponseData(errors.Wrap(err, "cannot init guild"))
	}

	return &api.InteractionResponseData{
		Flags:   discord.EphemeralMessage,
		Content: option.NewNullableString("Done!"),
	}
}

func (h *Handler) cmdMemberQuery(ctx context.Context, cmdData cmdroute.CommandData) *api.InteractionResponseData {
	guild, err := h.store.GuildInfo(cmdData.Event.GuildID)
	if err != nil {
		logger := logger.FromContext(h.ctx)
		logger.Println("ignoring guild", cmdData.Event.GuildID, "reason:", err)
		return nil
	}

	var data struct {
		Who discord.UserID `discord:"who"`
	}

	if err := cmdData.Options.Unmarshal(&data); err != nil {
		return ErrorResponseData(err)
	}

	metadata, err := h.store.MemberInfo(guild.GuildID, data.Who)
	if err != nil {
		return ErrorResponseData(err)
	}

	b, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return ErrorResponseData(errors.Wrap(err, "cannot encode metadata as JSON"))
	}

	return &api.InteractionResponseData{
		Flags:   discord.EphemeralMessage,
		Content: option.NewNullableString("```json\n" + string(b) + "\n```"),
	}
}

func (h *Handler) cmdMemberUnregister(ctx context.Context, cmdData cmdroute.CommandData) *api.InteractionResponseData {
	guild, err := h.store.GuildInfo(cmdData.Event.GuildID)
	if err != nil {
		logger := logger.FromContext(h.ctx)
		logger.Println("ignoring guild", cmdData.Event.GuildID, "reason:", err)
		return nil
	}

	var data struct {
		Who discord.UserID `discord:"who"`
	}

	if err := cmdData.Options.Unmarshal(&data); err != nil {
		return ErrorResponseData(err)
	}

	target, err := h.s.Member(cmdData.Event.GuildID, data.Who)
	if err != nil {
		return ErrorResponseData(errors.Wrap(err, "invalid member for 'who'"))
	}

	if err := h.store.UnregisterMember(cmdData.Event.GuildID, data.Who); err != nil {
		if errors.Is(err, acmregister.ErrNotFound) {
			err = errors.New("user is not registered")
		}
		return ErrorResponseData(err)
	}

	if err := h.s.RemoveRole(
		cmdData.Event.GuildID, data.Who, guild.RoleID,
		api.AuditLogReason(fmt.Sprintf(
			"%s requested for %s (%v) to be unregistered",
			cmdData.Event.Sender().Tag(), target.User.Tag(), data.Who,
		)),
	); err != nil {
		return ErrorResponseData(errors.Wrap(err, "cannot remove role, but member is registered"))
	}

	return &api.InteractionResponseData{
		Flags:           discord.EphemeralMessage,
		Content:         option.NewNullableString("User " + data.Who.Mention() + " has been unregistered."),
		AllowedMentions: &api.AllowedMentions{},
	}
}

func (h *Handler) cmdMemberResetName(ctx context.Context, cmdData cmdroute.CommandData) *api.InteractionResponseData {
	guild, err := h.store.GuildInfo(cmdData.Event.GuildID)
	if err != nil {
		logger := logger.FromContext(h.ctx)
		logger.Println("ignoring guild", cmdData.Event.GuildID, "reason:", err)
		return nil
	}

	var data struct {
		Who discord.UserID `discord:"who"`
	}

	if err := cmdData.Options.Unmarshal(&data); err != nil {
		return ErrorResponseData(err)
	}

	metadata, err := h.store.MemberInfo(guild.GuildID, data.Who)
	if err != nil {
		return ErrorResponseData(err)
	}

	if err := h.s.ModifyMember(cmdData.Event.GuildID, data.Who, api.ModifyMemberData{
		Nick: option.NewString(metadata.Nickname()),
	}); err != nil {
		return ErrorResponseData(err)
	}

	return &api.InteractionResponseData{
		Flags:           discord.EphemeralMessage,
		Content:         option.NewNullableString("User " + data.Who.Mention() + "'s nickname has been reset."),
		AllowedMentions: &api.AllowedMentions{},
	}
}

func (h *Handler) cmdClearRegistration(ctx context.Context, cmdData cmdroute.CommandData) *api.InteractionResponseData {
	_, err := h.store.GuildInfo(cmdData.Event.GuildID)
	if err != nil {
		logger := logger.FromContext(h.ctx)
		logger.Println("ignoring guild", cmdData.Event.GuildID, "reason:", err)
		return nil
	}

	if err := h.store.DeleteGuild(cmdData.Event.GuildID); err != nil {
		return ErrorResponseData(err)
	}

	return &api.InteractionResponseData{
		Flags:   discord.EphemeralMessage,
		Content: option.NewNullableString("Done. All members have been removed from the database, but their roles stay."),
	}
}

func (h *Handler) cmdEventExportMembers(ctx context.Context, cmdData cmdroute.CommandData) *api.InteractionResponseData {
	_, err := h.store.GuildInfo(cmdData.Event.GuildID)
	if err != nil {
		h.LogErr(cmdData.Event.GuildID, err)
		return ErrorResponseData(errors.New("guild is not registered"))
	}

	var data struct {
		Event string `discord:"event"`
	}
	if err := cmdData.Options.Unmarshal(&data); err != nil {
		return ErrorResponseData(err)
	}

	eventSnowflake, err := discord.ParseSnowflake(data.Event)
	if err != nil {
		return ErrorResponseData(errors.Wrap(err, "invalid event ID"))
	}
	eventID := discord.EventID(eventSnowflake)

	var members []api.GuildScheduledEventUser
	for {
		var after discord.UserID
		if len(members) > 0 {
			after = members[len(members)-1].User.ID
		}

		page, err := h.s.ListScheduledEventUsers(cmdData.Event.GuildID, eventID, nil, false, 0, after)
		if err != nil {
			h.LogErr(cmdData.Event.GuildID, errors.Wrap(err, "cannot list event users"))
			return InternalErrorResponseData()
		}
		if len(page) == 0 {
			break
		}

		members = append(members, page...)
	}

	type memberRecord struct {
		UserID    discord.UserID
		Username  string
		FirstName string
		LastName  string
		Email     string
	}

	memberRecords := make([]memberRecord, 0, len(members))
	var missed int

	for _, member := range members {
		record := memberRecord{
			UserID:   member.User.ID,
			Username: member.User.Username,
		}

		m, err := h.store.MemberInfo(cmdData.Event.GuildID, member.User.ID)
		if err == nil {
			record.FirstName = m.FirstName
			record.LastName = m.LastName
			record.Email = string(m.Email)
		} else {
			missed++
		}

		memberRecords = append(memberRecords, record)
	}

	var csvOut bytes.Buffer
	csvw := csv.NewWriter(&csvOut)
	csvw.Write([]string{"user_id", "username", "first_name", "last_name", "email"})
	if err := xcsv.Marshal(csvw, memberRecords); err != nil {
		return ErrorResponseData(errors.Wrap(err, "cannot write CSV"))
	}

	return &api.InteractionResponseData{
		Content: option.NewNullableString(fmt.Sprintf(""+
			"Here's a CSV file with **%d member(s)** exported.\n"+
			"A total of **%d member(s)** could not be found in the database.",
			len(members), missed,
		)),
		Files: []sendpart.File{
			{
				Name:   "participants.csv",
				Reader: bytes.NewReader(csvOut.Bytes()),
			},
		},
		AllowedMentions: &api.AllowedMentions{},
	}
}

var (
	eventsAutocompletionGroup = singleflight.Group[discord.GuildID, []discord.GuildScheduledEvent]{}
	eventsAutocompletionCache = ttlcache.New[discord.GuildID, []discord.GuildScheduledEvent]()
)

func (h *Handler) acEventExportMembers(ctx context.Context, acData cmdroute.AutocompleteData) api.AutocompleteChoices {
	client := h.s.WithContext(ctx)

	switch option := acData.Options.Focused(); option.Name {
	case "event":
		var events []discord.GuildScheduledEvent
		if item := eventsAutocompletionCache.Get(acData.Event.GuildID); item != nil {
			events = item.Value()
		} else {
			var err error
			events, err, _ = eventsAutocompletionGroup.Do(acData.Event.GuildID, func() ([]discord.GuildScheduledEvent, error) {
				events, err := client.ListScheduledEvents(acData.Event.GuildID, true)
				if err == nil {
					eventsAutocompletionCache.Set(acData.Event.GuildID, events, 15*time.Second)
				}
				return events, err
			})
			if err != nil {
				h.LogErr(acData.Event.GuildID, errors.Wrap(err, "cannot list events"))
				return nil
			}
		}

		eventQuery := strings.ToLower(option.String())

		var choices api.AutocompleteStringChoices
		for _, event := range events {
			if !strings.Contains(strings.ToLower(event.Name), eventQuery) {
				continue
			}
			choices = append(choices, discord.StringChoice{
				Name:  fmt.Sprintf("%s (%d interested)", event.Name, event.UserCount),
				Value: event.ID.String(),
			})
			if len(choices) == 25 {
				break
			}
		}

		return choices
	default:
		return nil
	}
}
