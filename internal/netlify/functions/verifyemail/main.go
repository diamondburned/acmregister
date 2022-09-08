package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/apex/gateway"
	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/bot"
	"github.com/diamondburned/acmregister/acmregister/env"
	"github.com/diamondburned/acmregister/acmregister/verifyemail"
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

	gateway.ListenAndServe("", srv)
	return nil
}

type handler struct {
	opts  *bot.Opts
	state *state.State
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Token    string                     `json:"token"`
		PIN      verifyemail.PIN            `json:"pin"`
		Metadata acmregister.MemberMetadata `json:"member_metadata"`
	}
}
