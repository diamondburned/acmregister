package bot

import (
	"strings"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/logger"
	"github.com/diamondburned/acmregister/acmregister/verifyemail"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/pkg/errors"
)

func (h *Handler) modalRegisterResponse(ev *discord.InteractionEvent, modal *discord.ModalInteraction) *api.InteractionResponse {
	guild, err := h.store.GuildInfo(ev.GuildID)
	if err != nil {
		logger := logger.FromContext(h.ctx)
		logger.Println("ignoring guild", ev.GuildID, "reason:", err)
		return nil
	}

	if _, err := h.store.MemberInfo(ev.GuildID, ev.SenderID()); err == nil {
		return ErrorResponse(errors.New("you're already registered!"))
	}

	var data struct {
		Email     acmregister.Email    `discord:"email"`
		FirstName string               `discord:"first"`
		LastName  string               `discord:"last?"`
		Pronouns  acmregister.Pronouns `discord:"pronouns?"`
	}

	if err := modal.Components.Unmarshal(&data); err != nil {
		return ErrorResponse(err)
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
		h.LogErr(ev.GuildID, errors.Wrap(err, "cannot save registration submission (not important)"))
		// not important so we continue
	}

	if err := metadata.Pronouns.Validate(); err != nil {
		return ErrorResponse(err)
	}

	if err := h.opts.verifyEmail(h.ctx, metadata.Email); err != nil {
		return ErrorResponse(err)
	}

	if h.opts.EmailScheduler == nil {
		return h.registerAndRespond(ev, guild, metadata)
	}

	if err := h.opts.EmailScheduler.ScheduleConfirmationEmail(&h.Client, ev, member); err != nil {
		h.LogErr(ev.GuildID, errors.Wrap(err, "cannot schedule confirmation email"))
		return InternalErrorResponse()
	}

	return deferResponse(discord.EphemeralMessage)
}

func (h *Handler) modalVerifyPIN(ev *discord.InteractionEvent, modal *discord.ModalInteraction) *api.InteractionResponse {
	if h.opts.EmailScheduler == nil {
		logger := logger.FromContext(h.ctx)
		logger.Println("error: verify-pin invoked without opts.EmailScheduler")
		return InternalErrorResponse()
	}

	guild, err := h.store.GuildInfo(ev.GuildID)
	if err != nil {
		logger := logger.FromContext(h.ctx)
		logger.Println("ignoring guild", ev.GuildID, "reason:", err)
		return nil
	}

	var data struct {
		PIN verifyemail.PIN `discord:"pin"`
	}

	if err := modal.Components.Unmarshal(&data); err != nil {
		return ErrorResponse(err)
	}

	metadata, err := h.opts.PINStore.ValidatePIN(ev.GuildID, ev.SenderID(), data.PIN)
	if err != nil {
		// Warn about weird errors just in case.
		if err != nil && !errors.Is(err, acmregister.ErrNotFound) {
			h.PrivateWarning(ev, errors.Wrap(err, "cannot validate PIN"))
		}

		return ErrorResponse(errors.New("incorrect PIN code given, try again"))
	}

	// At this point, the user ID matches with the known email, and the given
	// PIN also matches that email, so we're good.
	return h.registerAndRespond(ev, guild, *metadata)
}

func (h *Handler) registerAndRespond(ev *discord.InteractionEvent, guild *acmregister.KnownGuild, metadata acmregister.MemberMetadata) *api.InteractionResponse {
	member := acmregister.Member{
		GuildID:  ev.GuildID,
		UserID:   ev.SenderID(),
		Metadata: metadata,
	}

	if err := h.store.RegisterMember(member); err != nil && !errors.Is(err, acmregister.ErrMemberAlreadyExists) {
		h.PrivateWarning(ev, errors.Wrap(err, "cannot save into database"))
		return InternalErrorResponse()
	}

	return h.assignThenRespond(ev, guild, metadata)
}

func (h *Handler) assignThenRespond(ev *discord.InteractionEvent, guild *acmregister.KnownGuild, metadata acmregister.MemberMetadata) *api.InteractionResponse {
	if err := h.s.AddRole(guild.GuildID, ev.SenderID(), guild.RoleID, api.AddRoleData{
		AuditLogReason: "member registered, added by acmRegister",
	}); err != nil {
		h.PrivateWarning(ev, errors.Wrap(err, "cannot add role"))
		return InternalErrorResponse()
	}

	if err := h.s.ModifyMember(guild.GuildID, ev.SenderID(), api.ModifyMemberData{
		Nick: option.NewString(metadata.Nickname()),
	}); err != nil {
		h.PrivateWarning(ev, errors.Wrap(err, "cannot nickname new member (not important)"))
	}

	msg := guild.RegisteredMessage
	if msg == "" {
		msg = registeredMessage
	}
	return msgResponse(&api.InteractionResponseData{
		Flags:   discord.EphemeralMessage,
		Content: option.NewNullableString(msg),
	})
}
