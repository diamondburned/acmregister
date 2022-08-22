package bot

import (
	"strings"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/verifyemail"
	"github.com/diamondburned/acmregister/internal/logger"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/pkg/errors"
)

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
		Email     acmregister.Email    `discord:"email"`
		FirstName string               `discord:"first"`
		LastName  string               `discord:"last?"`
		Pronouns  acmregister.Pronouns `discord:"pronouns?"`
	}

	if err := modal.Components.Unmarshal(&data); err != nil {
		h.sendErr(ev, err)
		return
	}

	metadata := acmregister.MemberMetadata(data)
	metadata.Email = acmregister.Email(strings.TrimSpace(string(metadata.Email)))
	metadata.FirstName = strings.TrimSpace(metadata.FirstName)
	metadata.LastName = strings.TrimSpace(metadata.LastName)

	member := acmregister.Member{
		GuildID:  ev.GuildID,
		UserID:   ev.SenderID(),
		Metadata: metadata,
	}

	if err := h.store.SaveSubmission(member); err != nil {
		h.logErr(ev.GuildID, errors.Wrap(err, "cannot save registration submission (not important)"))
		// not important so we continue
	}

	if err := metadata.Pronouns.Validate(); err != nil {
		h.sendErr(ev, err)
		return
	}

	if err := h.opts.verifyEmail(h.ctx, metadata.Email); err != nil {
		h.sendErr(ev, err)
		return
	}

	if h.opts.SMTPVerifier == nil {
		h.registerAndRespond(ev, guild, metadata)
		return
	}

	// This might take a while.
	respond := h.deferResponse(ev, discord.EphemeralMessage)

	if err := h.opts.SMTPVerifier.SendConfirmationEmail(h.ctx, member); err != nil {
		h.privateWarning(ev, errors.Wrap(err, "cannot send confirmation email"))
		respond(errorResponseData(errors.New("we cannot send you an email, please contact the server administrator")))
		return
	}

	respond(&api.InteractionResponseData{
		Flags:   discord.EphemeralMessage,
		Content: option.NewNullableString(verifyPINMessage),
		Components: &discord.ContainerComponents{
			&discord.ActionRowComponent{
				&discord.ButtonComponent{
					Style:    discord.PrimaryButtonStyle(),
					CustomID: "verify-pin",
					Label:    verifyPINButtonLabel,
				},
			},
		},
	})
}

func (h *Handler) modalVerifyPIN(ev *gateway.InteractionCreateEvent, modal *discord.ModalInteraction) {
	guild, err := h.store.GuildInfo(ev.GuildID)
	if err != nil {
		logger := logger.FromContext(h.ctx)
		logger.Println("ignoring unknown guild", ev.GuildID)
		return
	}

	if h.opts.SMTPVerifier == nil {
		metadata, err := h.store.RestoreSubmission(ev.GuildID, ev.SenderID())
		if err != nil {
			h.sendErr(ev, errors.Wrap(err, "cannot restore your registration, try registering again"))
			return
		}

		// Just in case the user manually triggered this interaction when this
		// feature is disabled. We don't want to crash the whole bot.
		h.registerAndRespond(ev, guild, *metadata)
		return
	}

	var data struct {
		PIN verifyemail.PIN `discord:"pin"`
	}

	if err := modal.Components.Unmarshal(&data); err != nil {
		h.sendErr(ev, err)
		return
	}

	metadata, err := h.opts.SMTPVerifier.ValidatePIN(ev.GuildID, ev.SenderID(), data.PIN)
	if err != nil {
		// Warn about weird errors just in case.
		if err != nil && !errors.Is(err, acmregister.ErrNotFound) {
			h.privateWarning(ev, errors.Wrap(err, "cannot validate PIN"))
		}

		h.sendErr(ev, errors.New("incorrect PIN code given, try again"))
		return
	}

	// At this point, the user ID matches with the known email, and the given
	// PIN also matches that email, so we're good.
	h.registerAndRespond(ev, guild, *metadata)
}

func (h *Handler) registerAndRespond(ev *gateway.InteractionCreateEvent, guild *acmregister.KnownGuild, metadata acmregister.MemberMetadata) {
	member := acmregister.Member{
		GuildID:  ev.GuildID,
		UserID:   ev.SenderID(),
		Metadata: metadata,
	}

	if err := h.store.RegisterMember(member); err != nil {
		if errors.Is(err, acmregister.ErrMemberAlreadyExists) {
			h.sendErr(ev, acmregister.ErrMemberAlreadyExists)
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

	msg := guild.RegisteredMessage
	if msg == "" {
		msg = registeredMessage
	}

	h.respondInteraction(ev, &api.InteractionResponseData{
		Flags:   discord.EphemeralMessage,
		Content: option.NewNullableString(msg),
	})
}
