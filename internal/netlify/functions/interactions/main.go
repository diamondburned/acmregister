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

	serverVars := env.InteractionServer()

	return gateway.ListenAndServe("", handler{
		discord: state.NewAPIOnlyState(botToken, nil),
		opts:    envOpts.Opts,
		svars:   serverVars,
	})
}

type handler struct {
	discord *state.State
	opts    bot.Opts
	svars   env.InteractionServerVars
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	scheme := "https://"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto + "://"
	}

	// Switcheroo: inject this for /verifyemail to work. We just trick the
	// Discord handler into thinking we sent an email, but we actually just
	// delegated that to another lambda!
	//
	// We do this because we can't just use regular goroutines, because that
	// would make too much sense and AWS can't have it.
	opts := h.opts
	opts.EmailScheduler = confirmationEmailScheduler{
		url: scheme + r.Host,
		ctx: r.Context(),
	}

	handler := bot.NewHandler(h.discord, opts)

	srv, err := servutil.NewInteractionServer(h.svars.PubKey, handler)
	if err != nil {
		servutil.WriteErr(w, r,
			http.StatusInternalServerError, errors.Wrap(err, "cannot create interaction server"))
		return
	}

	srv.ServeHTTP(w, r)
}
