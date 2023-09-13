package tracer

import (
	"github.com/diamondburned/arikawa/v3/api/cmdroute"
	"github.com/diamondburned/arikawa/v3/api/webhook"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
)

// TraceCommandRouter injects tracing into a cmdroute.Router.
// The name is used as the span name.
// A new handler is returned that should be used instead of the original.
func TraceCommandRouter(name string, r *cmdroute.Router) webhook.InteractionHandler {
	r.Use(func(next cmdroute.InteractionHandler) cmdroute.InteractionHandler {
		return cmdroute.InteractionHandlerFunc(func(ev *discord.InteractionEvent) *api.InteractionResponse {
			span := tracer.StartSpan(name + "-handler")
			defer span.Finish()
			return next.HandleInteraction(ev)
		})
	})

	return cmdroute.InteractionHandlerFunc(func(ev *discord.InteractionEvent) *api.InteractionResponse {
		span := tracer.StartSpan(name + "-router")
		defer span.Finish()
		return r.HandleInteraction(ev)
	})
}
