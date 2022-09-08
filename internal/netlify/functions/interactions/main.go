package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"

	// TODO: this might be converting requests to GET
	"github.com/apex/gateway/v2"
	"github.com/aws/aws-lambda-go/lambda"
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

	gw := gateway.NewGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("HTTP method =", r.Method)
		srv.ServeHTTP(w, r)
	}))

	lambda.StartHandler(debugh{
		h: gw,
	})
	return nil
}

type debugh struct {
	h lambda.Handler
}

func (h debugh) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	log.Println(string(payload))
	return h.h.Invoke(ctx, payload)
}
