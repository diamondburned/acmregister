package bot

import (
	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/logger"
	"github.com/diamondburned/acmregister/acmregister/verifyemail"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
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
					Value:        string(data.Email),
					Placeholder:  h.opts.EmailHosts.String() + " only",
					Style:        discord.TextInputShortStyle,
					Required:     true,
					LengthLimits: [2]int{0, 150},
				},
			},
			&discord.ActionRowComponent{
				&discord.TextInputComponent{
					CustomID:     "first",
					Label:        "First Name",
					Value:        data.FirstName,
					Style:        discord.TextInputShortStyle,
					Required:     true,
					LengthLimits: [2]int{0, 45},
				},
			},
			&discord.ActionRowComponent{
				&discord.TextInputComponent{
					CustomID:     "last",
					Label:        "Last Name (optional)",
					Value:        data.LastName,
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
					Value:        string(data.Pronouns),
					Placeholder:  "he/him, she/her, they/them, or any",
				},
			},
		},
	}
}

func (h *Handler) buttonRegister(ev *discord.InteractionEvent) *api.InteractionResponse {
	guild, err := h.store.GuildInfo(ev.GuildID)
	if err != nil {
		logger := logger.FromContext(h.ctx)
		logger.Println("ignoring guild", ev.GuildID, "reason:", err)
		return nil
	}

	if metadata, err := h.store.MemberInfo(ev.GuildID, ev.SenderID()); err == nil {
		// Member already registered. Just assign the role.
		return h.assignThenRespond(ev, guild, *metadata)
	}

	metadata, err := h.store.RestoreSubmission(ev.GuildID, ev.SenderID())
	if err != nil {
		if !errors.Is(err, acmregister.ErrNotFound) {
			h.LogErr(ev.GuildID, errors.Wrap(err, "failed to restore submission"))
		}
		metadata = &acmregister.MemberMetadata{}
	}

	return &api.InteractionResponse{
		Type: api.ModalResponse,
		Data: h.makeRegisterModal(*metadata),
	}
}

var verifyPINModal = &api.InteractionResponseData{
	CustomID: option.NewNullableString("verify-pin"),
	Title:    option.NewNullableString("Verify your PIN code"),
	Components: &discord.ContainerComponents{
		&discord.ActionRowComponent{
			&discord.TextInputComponent{
				CustomID:     "pin",
				Label:        "PIN code",
				Placeholder:  verifyemail.InvalidPIN.Format(),
				Style:        discord.TextInputShortStyle,
				Required:     true,
				LengthLimits: [2]int{verifyemail.PINDigits, verifyemail.PINDigits},
			},
		},
	},
}

func (h *Handler) buttonVerifyPIN(ev *discord.InteractionEvent) *api.InteractionResponse {
	_, err := h.store.GuildInfo(ev.GuildID)
	if err != nil {
		logger := logger.FromContext(h.ctx)
		logger.Println("ignoring guild", ev.GuildID, "reason:", err)
		return nil
	}

	_, err = h.store.RestoreSubmission(ev.GuildID, ev.SenderID())
	if err != nil {
		return ErrorResponse(errors.New("you haven't started registering yet"))
	}

	return &api.InteractionResponse{
		Type: api.ModalResponse,
		Data: verifyPINModal,
	}
}
