package acmregister

import (
	"fmt"

	"github.com/diamondburned/acmregister/internal/logger"
	"github.com/diamondburned/acmregister/internal/shibboleth"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/pkg/errors"
)

func makeRegisterModal(data MemberMetadata) *api.InteractionResponseData {
	return &api.InteractionResponseData{
		CustomID: option.NewNullableString("register-response"),
		Title:    option.NewNullableString("Register"),
		Content:  option.NewNullableString("Register as a member of acmCSUF here!"),
		Components: &discord.ContainerComponents{
			&discord.ActionRowComponent{
				&discord.TextInputComponent{
					CustomID:     "email",
					Label:        "Email",
					Value:        option.NewNullableString(data.Email),
					Placeholder:  option.NewNullableString(AllowedEmailDomainsLabel() + " only"),
					Style:        discord.TextInputShortStyle,
					Required:     true,
					LengthLimits: [2]int{0, 150},
				},
			},
			&discord.ActionRowComponent{
				&discord.TextInputComponent{
					CustomID:     "first",
					Label:        "First Name",
					Value:        option.NewNullableString(data.FirstName),
					Style:        discord.TextInputShortStyle,
					Required:     true,
					LengthLimits: [2]int{0, 45},
				},
			},
			&discord.ActionRowComponent{
				&discord.TextInputComponent{
					CustomID:     "last",
					Label:        "Last Name (optional)",
					Value:        option.NewNullableString(data.LastName),
					Style:        discord.TextInputShortStyle,
					LengthLimits: [2]int{0, 45},
				},
			},
			&discord.ActionRowComponent{
				&discord.TextInputComponent{
					CustomID:     "pronouns",
					Label:        "Pronouns (optional)",
					Style:        discord.TextInputShortStyle,
					Required:     false,
					LengthLimits: [2]int{0, 45},
					Value:        option.NewNullableString(string(data.Pronouns)),
					Placeholder:  option.NewNullableString("he/him, she/her, they/them, or any"),
				},
			},
		},
	}
}

func (h *Handler) modalRegisterResponse(ev *gateway.InteractionCreateEvent, modal *discord.ModalInteraction) {
	guild, err := h.store.GuildInfo(ev.GuildID)
	if err != nil {
		logger := logger.FromContext(h.ctx)
		logger.Println("ignoring unknown guild", ev.GuildID)
		return
	}

	if _, err := h.store.MemberInfo(ev.GuildID, ev.SenderID()); err == nil {
		h.sendErr(ev, errors.New("you're already registered!"))
		return
	}

	var data struct {
		Email     string   `discord:"email"`
		FirstName string   `discord:"first"`
		LastName  string   `discord:"last?"`
		Pronouns  Pronouns `discord:"pronouns?"`
	}

	if err := modal.Components.Unmarshal(&data); err != nil {
		h.sendErr(ev, err)
		return
	}

	metadata := MemberMetadata(data)

	if err := h.store.SaveSubmission(ev.GuildID, ev.SenderID(), metadata); err != nil {
		h.logErr(ev.GuildID, errors.Wrap(err, "cannot save registration submission (not important)"))
		// not important so we continue
	}

	if err := metadata.Validate(); err != nil {
		h.sendErr(ev, err)
		return
	}

	validEmail, err := shibboleth.IsValidUser(h.ctx, ShibbolethURL, metadata.EmailUsername())
	if err != nil {
		h.privateErr(ev, errors.Wrap(err, "cannot validate user email"))
		return
	}

	if !validEmail {
		h.sendErr(ev, fmt.Errorf("your email is not in the CSU Fullerton registry"))
		return
	}

	if err := h.store.RegisterMember(ev.GuildID, ev.SenderID(), metadata); err != nil {
		if errors.Is(err, ErrMemberAlreadyExists) {
			h.sendErr(ev, ErrMemberAlreadyExists)
		} else {
			h.privateErr(ev, errors.Wrap(err, "cannot save into database"))
		}
		return
	}

	if err := h.s.AddRole(ev.GuildID, ev.SenderID(), guild.RoleID, api.AddRoleData{
		AuditLogReason: "member registered, added by acmRegister",
	}); err != nil {
		h.privateErr(ev, errors.Wrap(err, "cannot add role"))
		return
	}

	if err := h.s.ModifyMember(ev.GuildID, ev.SenderID(), api.ModifyMemberData{
		Nick: option.NewString(metadata.Nickname()),
	}); err != nil {
		h.privateWarning(ev, errors.Wrap(err, "cannot nickname new member (not important)"))
	}

	if guild.RegisteredMessage != "" {
		h.respondInteraction(ev, &api.InteractionResponseData{
			Content: option.NewNullableString(guild.RegisteredMessage),
			Flags:   api.EphemeralResponse,
		})
	}
}
