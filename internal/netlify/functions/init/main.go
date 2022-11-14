package main

import (
	"context"
	"log"
	"net/http"

	"github.com/apex/gateway"
	"github.com/diamondburned/acmregister/acmregister/bot"
	"github.com/diamondburned/acmregister/acmregister/env"
	"github.com/diamondburned/acmregister/internal/netlify/servutil"
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

	return gateway.ListenAndServe("", handler{
		Handler: bot.NewHandler(
			state.NewAPIOnlyState(botToken, nil),
			envOpts.Opts,
		),
	})
}

type handler struct {
	*bot.Handler
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.OverwriteCommands(); err != nil {
		servutil.WriteErr(w, r,
			http.StatusInternalServerError, errors.Wrap(err, "cannot create interaction server"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Done."))
}
