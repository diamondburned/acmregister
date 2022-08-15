package acmregister

import (
	"context"
	"fmt"

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

// Bind binds acmregister commands to s.
func Bind(s *state.State, store Store) error {
	h := NewHandler(s, store)
	if err := h.OverwriteCommands(); err != nil {
		return err
	}

	s.AddHandler(h.OnInteractionCreateEvent)
	s.AddIntents(gateway.IntentGuilds)
	s.AddIntents(gateway.IntentDirectMessages)

	return nil
}

type Handler struct {
	s     *state.State
	ctx   context.Context
	store Store
	bound bool
}

// NewHandler creates a new Handler instance bound to the given State.
func NewHandler(s *state.State, store Store) *Handler {
	return &Handler{
		s:     s,
		ctx:   s.Context(),
		store: store.WithContext(s.Context()),
	}
}

func (h *Handler) OnInteractionCreateEvent(ev *gateway.InteractionCreateEvent) {
	defer func() {
		if panicked := recover(); panicked != nil {
			h.sendErr(ev, fmt.Errorf("bug: panic occured: %v", panicked))
		}
	}()

	switch data := ev.Data.(type) {
	case *discord.CommandInteraction:
		p, err := h.s.Permissions(ev.ChannelID, ev.SenderID())
		if err != nil {
			h.sendErr(ev, errors.Wrap(err, "cannot get permission for yourself"))
			return
		}

		// Limit all commands to admins only.
		if !p.Has(discord.PermissionAdministrator) {
			h.sendErr(ev, fmt.Errorf("you're not an administrator; contact the guild owner"))
			return
		}

		switch data.Name {
		case "init-register":
			h.cmdInit(ev, data.Options)
		case "registered-member":
			if len(data.Options) == 1 {
				switch data := data.Options[0]; data.Name {
				case "query":
					h.cmdMemberQuery(ev, data.Options)
				case "unregister":
					h.cmdMemberUnregister(ev, data.Options)
				}
			}
		case "clear-registration":
			h.cmdClear(ev, data.Options)
		default:
			logger := logger.FromContext(h.ctx)
			logger.Printf("not handling unknown command %q", data.Name)
		}
	case *discord.ButtonInteraction:
		switch data.CustomID {
		case "register":
			h.buttonRegister(ev)
		default:
			logger := logger.FromContext(h.ctx)
			logger.Printf("not handling unknown button %q", data.CustomID)
		}
	case *discord.ModalInteraction:
		switch data.CustomID {
		case "register-response":
			h.modalRegisterResponse(ev, data)
		default:
			logger := logger.FromContext(h.ctx)
			logger.Printf("not handling unknown modal %q", data.CustomID)
		}
	default:
		logger := logger.FromContext(h.ctx)
		logger.Printf("not handling unknown command type %T", data)
	}
}

func (h *Handler) respond(ev *gateway.InteractionCreateEvent, resp api.InteractionResponse) {
	if err := h.s.RespondInteraction(ev.ID, ev.Token, resp); err != nil {
		err = errors.Wrap(err, "cannot respond to interaction")
		// DO NOT CALL h.sendErr HERE!! It has the possibility of recursing
		// forever!!
		h.logErr(err)
		h.sendDMErr(ev, err)
	}
}

func (h *Handler) respondInteraction(ev *gateway.InteractionCreateEvent, data *api.InteractionResponseData) {
	h.respond(ev, api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: data,
	})
}

func (h *Handler) sendErr(ev *gateway.InteractionCreateEvent, sendErr error) {
	h.respondInteraction(ev, &api.InteractionResponseData{
		Content: option.NewNullableString("⚠️ **Error:** " + sendErr.Error()),
		Flags:   api.EphemeralResponse,
	})
}

// privateErr should be used for private, secret or internal-only errors. The
// user need not to know about these errors, so they'll get an ambiguous message.
func (h *Handler) privateErr(ev *gateway.InteractionCreateEvent, sendErr error) {
	h.logErr(sendErr)
	h.sendErr(ev, errors.New("internal error occured, please contact the server administrator"))
	h.sendDMErr(ev, sendErr)
}

func (h *Handler) sendDMErr(ev *gateway.InteractionCreateEvent, sendErr error) {
	guild, err := h.store.GuildInfo(ev.GuildID)
	if err != nil {
		h.logErr(err)
		return
	}

	dm, err := h.s.CreatePrivateChannel(guild.InitUserID)
	if err != nil {
		h.logErr(err)
		return
	}

	if _, err = h.s.SendMessage(dm.ID, "⚠️ Error: "+sendErr.Error()); err != nil {
		h.logErr(errors.Wrap(err, "cannot send error to DM"))
		return
	}
}

func (h *Handler) logErr(err error) {
	logger := logger.FromContext(h.ctx)
	logger.Println("command error:", err)
}
