package bot

import (
	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/verifyemail"
	"github.com/diamondburned/acmregister/internal/logger"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/pkg/errors"
)

func (h *Handler) makeRegisterModal(data acmregister.MemberMetadata) *api.InteractionResponseData {
	return &api.InteractionResponseData{
		CustomID: option.NewNullableString("register-response"),
		Title:    option.NewNullableString("Register"),
		Components: &discord.ContainerComponents{
			&discord.ActionRowComponent{
				&discord.TextInputComponent{
					CustomID:     "email",
					Label:        "Email",
					Value:        option.NewNullableString(string(data.Email)),
					Placeholder:  option.NewNullableString(h.opts.EmailHosts.String() + " only"),
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

func (h *Handler) buttonRegister(ev *gateway.InteractionCreateEvent) {
	_, err := h.store.GuildInfo(ev.GuildID)
	if err != nil {
		logger := logger.FromContext(h.ctx)
		logger.Println("ignoring unknown guild", ev.GuildID)
		return
	}

	if _, err := h.store.MemberInfo(ev.GuildID, ev.SenderID()); err == nil {
		h.sendErr(ev, errors.New("you're already registered!"))
		return
	}

	metadata, _ := h.store.RestoreSubmission(ev.GuildID, ev.SenderID())
	if metadata == nil {
		metadata = &acmregister.MemberMetadata{}
	}

	h.respond(ev, api.InteractionResponse{
		Type: api.ModalResponse,
		Data: h.makeRegisterModal(*metadata),
	})
}

var verifyPINModal = &api.InteractionResponseData{
	CustomID: option.NewNullableString("verify-pin"),
	Title:    option.NewNullableString("Verify your PIN code"),
	Components: &discord.ContainerComponents{
		&discord.ActionRowComponent{
			&discord.TextInputComponent{
				CustomID:     "pin",
				Label:        "PIN code",
				Placeholder:  option.NewNullableString(verifyemail.InvalidPIN.Format()),
				Style:        discord.TextInputShortStyle,
				Required:     true,
				LengthLimits: [2]int{verifyemail.PINDigits, verifyemail.PINDigits},
			},
		},
	},
}

func (h *Handler) buttonVerifyPIN(ev *gateway.InteractionCreateEvent) {
	_, err := h.store.GuildInfo(ev.GuildID)
	if err != nil {
		logger := logger.FromContext(h.ctx)
		logger.Println("ignoring unknown guild", ev.GuildID)
		return
	}

	_, err = h.store.RestoreSubmission(ev.GuildID, ev.SenderID())
	if err != nil {
		h.sendErr(ev, errors.New("you haven't started registering yet"))
		return
	}

	h.respond(ev, api.InteractionResponse{
		Type: api.ModalResponse,
		Data: verifyPINModal,
	})
}
