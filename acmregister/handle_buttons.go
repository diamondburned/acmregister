package acmregister

import (
	"github.com/diamondburned/acmregister/internal/logger"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/pkg/errors"
)

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
		metadata = &MemberMetadata{}
	}

	h.respond(ev, api.InteractionResponse{
		Type: api.ModalResponse,
		Data: makeRegisterModal(*metadata),
	})
}
