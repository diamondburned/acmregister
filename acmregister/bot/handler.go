package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/verifyemail"
	"github.com/diamondburned/acmregister/internal/logger"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/pkg/errors"
)

// TODO: if member is already registered, just give them the role
// TODO: command to migrate roles

type Opts struct {
	Store      acmregister.Store
	EmailHosts acmregister.EmailHostsVerifier
	// ShibbolethVerifier is optional.
	ShibbolethVerifier *verifyemail.ShibbolethVerifier
	// SMTPVerifier is optional.
	SMTPVerifier *verifyemail.SMTPVerifier
}

func (o Opts) verifyEmail(ctx context.Context, email acmregister.Email) error {
	if o.EmailHosts != nil {
		if err := o.EmailHosts.Verify(email); err != nil {
			return err
		}
	}

	if o.ShibbolethVerifier != nil {
		if err := o.ShibbolethVerifier.Verify(ctx, email); err != nil {
			return err
		}
	}

	return nil
}

type Handler struct {
	s     *state.State
	ctx   context.Context
	opts  Opts
	store acmregister.Store
	bound bool
}

// NewHandler creates a new Handler instance bound to the given State.
func NewHandler(s *state.State, opts Opts) *Handler {
	return &Handler{
		s:     s,
		ctx:   s.Context(),
		opts:  opts,
		store: opts.Store.WithContext(s.Context()).(acmregister.Store),
	}
}

func (h *Handler) Intents() gateway.Intents {
	return 0 |
		gateway.IntentGuilds |
		gateway.IntentDirectMessages
}

func (h *Handler) HandleInteraction(ev *discord.InteractionEvent) *api.InteractionResponse {
	defer func() {
		if panicked := recover(); panicked != nil {
			h.privateWarning(ev, fmt.Errorf("bug: panic occured: %v", panicked))
		}
	}()

	switch data := ev.Data.(type) {
	case *discord.CommandInteraction:
		p, err := h.s.Permissions(ev.ChannelID, ev.SenderID())
		if err != nil {
			return errorResponse(errors.Wrap(err, "cannot get permission for yourself"))
		}

		// Limit all commands to admins only.
		if !p.Has(discord.PermissionAdministrator) {
			return errorResponse(fmt.Errorf("you're not an administrator; contact the guild owner"))
		}

		switch data.Name {
		case "init-register":
			return h.cmdInit(ev, data.Options)
		case "registered-member":
			if len(data.Options) == 1 {
				switch data := data.Options[0]; data.Name {
				case "query":
					return h.cmdMemberQuery(ev, data.Options)
				case "unregister":
					return h.cmdMemberUnregister(ev, data.Options)
				case "reset-name":
					return h.cmdMemberResetName(ev, data.Options)
				}
			}
		case "clear-registration":
			return h.cmdClear(ev, data.Options)
		default:
			logger := logger.FromContext(h.ctx)
			logger.Printf("not handling unknown command %q", data.Name)
		}
	case *discord.ButtonInteraction:
		switch data.CustomID {
		case "register":
			return h.buttonRegister(ev)
		case "verify-pin":
			return h.buttonVerifyPIN(ev)
		default:
			logger := logger.FromContext(h.ctx)
			logger.Printf("not handling unknown button %q", data.CustomID)
		}
	case *discord.ModalInteraction:
		switch data.CustomID {
		case "register-response":
			return h.modalRegisterResponse(ev, data)
		case "verify-pin":
			return h.modalVerifyPIN(ev, data)
		default:
			logger := logger.FromContext(h.ctx)
			logger.Printf("not handling unknown modal %q", data.CustomID)
		}
	case *discord.PingInteraction:
		// ignore
	default:
		logger := logger.FromContext(h.ctx)
		logger.Printf("not handling unknown command type %T", data)
	}

	return nil
}

func (h *Handler) followUp(ev *discord.InteractionEvent, data *api.InteractionResponseData) {
	// Try for a few seconds.
	ctx, cancel := context.WithTimeout(h.ctx, 3*time.Second)
	defer cancel()

	s := h.s.WithContext(ctx)
	var err error

	for ctx.Err() == nil {
		_, err = s.FollowUpInteraction(ev.AppID, ev.Token, *data)
		if err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	if err != nil {
		err = errors.Wrap(err, "cannot follow-up to interaction")
		h.logErr(ev.GuildID, err)
		h.sendDMErr(ev, err)
	}
}

// privateWarning is like privateErr, except the user does not get a reply back
// saying things have gone wrong. Use this if we don't intend to return after
// the error.
func (h *Handler) privateWarning(ev *discord.InteractionEvent, sendErr error) {
	h.logErr(ev.GuildID, sendErr)
	h.sendDMErr(ev, sendErr)
}

func (h *Handler) sendDMErr(ev *discord.InteractionEvent, sendErr error) {
	guild, err := h.store.GuildInfo(ev.GuildID)
	if err != nil {
		h.logErr(ev.GuildID, err)
		return
	}

	dm, err := h.s.CreatePrivateChannel(guild.InitUserID)
	if err != nil {
		h.logErr(ev.GuildID, err)
		return
	}

	if _, err = h.s.SendMessage(dm.ID, "⚠️ Error: "+sendErr.Error()); err != nil {
		h.logErr(ev.GuildID, errors.Wrap(err, "cannot send error to DM"))
		return
	}
}

func (h *Handler) logErr(guildID discord.GuildID, err error) {
	var guildInfo string
	if guild, err := h.s.Guild(guildID); err == nil {
		guildInfo = fmt.Sprintf("%q (%d)", guild.Name, guild.ID)
	} else {
		guildInfo = fmt.Sprintf("%d", guildID)
	}

	logger := logger.FromContext(h.ctx)
	logger.Println("guild "+guildInfo+":", "command error:", err)
}

func msgResponse(data *api.InteractionResponseData) *api.InteractionResponse {
	return &api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: data,
	}
}

func deferResponse(flags discord.MessageFlags) *api.InteractionResponse {
	return &api.InteractionResponse{
		Type: api.DeferredMessageInteractionWithSource,
		Data: &api.InteractionResponseData{
			Flags: flags,
		},
	}
}

func internalErrorResponse() *api.InteractionResponse {
	return errorResponse(errors.New("internal error occured, please contact the server administrator"))
}

func errorResponse(err error) *api.InteractionResponse {
	return &api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: errorResponseData(err),
	}
}

func errorResponseData(err error) *api.InteractionResponseData {
	return &api.InteractionResponseData{
		Content: option.NewNullableString("⚠️ **Error:** " + err.Error()),
		Flags:   discord.EphemeralMessage,
	}
}
