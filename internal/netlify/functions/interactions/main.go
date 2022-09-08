package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/akrylysov/algnhsa"
	"github.com/diamondburned/acmregister/acmregister/bot"
	"github.com/diamondburned/acmregister/acmregister/env"
	"github.com/diamondburned/acmregister/internal/netlify/servutil"
	"github.com/diamondburned/arikawa/v3/api/webhook"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/pkg/errors"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Fatalln(err)
	}
}

func run(ctx context.Context) error {
	botToken, err := env.BotToken()
	if err != nil {
		return errors.Wrap(err, "cannot get bot token")
	}

	opts, err := env.BotOpts(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot init bot opts")
	}
	defer opts.Store.Close()

	state := state.NewAPIOnlyState(botToken, nil).WithContext(ctx)

	handler := bot.NewHandler(state, opts)
	defer handler.Close()

	serverVars := env.InteractionServer()

	srv, err := webhook.NewInteractionServer(serverVars.PubKey, handler)
	if err != nil {
		return errors.Wrap(err, "cannot create interaction server")
	}
	srv.ErrorFunc = servutil.WriteErr

	algnhsa.ListenAndServe(srv, nil)
	return nil
}
