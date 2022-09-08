package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/apex/gateway"
	"github.com/diamondburned/acmregister/acmregister/bot"
	"github.com/diamondburned/acmregister/acmregister/env"
	"github.com/diamondburned/acmregister/internal/netlify/api"
	"github.com/diamondburned/acmregister/internal/netlify/servutil"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/pkg/errors"
)

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	botToken, err := env.BotToken()
	if err != nil {
		return errors.Wrap(err, "cannot get bot token")
	}

	envOpts, err := env.BotOpts(context.Background())
	if err != nil {
		return errors.Wrap(err, "cannot init bot opts")
	}
	defer envOpts.Store.Close()

	if envOpts.SMTPVerifier == nil {
		return errors.Wrap(err, "/verifyemail called without SMTPVerifier in env")
	}

	return gateway.ListenAndServe("", handler{
		discord: state.NewAPIOnlyState(botToken, nil),
		opts:    envOpts,
	})
}

type handler struct {
	discord *state.State
	opts    env.Opts
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var data api.VerifyEmailData

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		servutil.WriteErr(w, r, http.StatusBadRequest, err)
		return
	}

	// Make shit up. I should've designed the data more thoroughly, but the
	// functions below won't even use most of these fields.
	ev := &discord.InteractionEvent{
		AppID:   data.AppID,
		Token:   data.Token,
		GuildID: data.Member.GuildID,
	}

	client := bot.NewClient(r.Context(), h.discord)

	bot.SendConfirmationEmail(r.Context(), h.opts.SMTPVerifier, client, ev, data.Member)
}
