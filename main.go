package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/bot"
	"github.com/diamondburned/acmregister/acmregister/verifyemail"
	"github.com/diamondburned/acmregister/internal/stores"
	"github.com/diamondburned/arikawa/v3/api/webhook"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/listener"
)

func init() { rand.Seed(time.Now().UnixNano()) }

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatalln("no $BOT_TOKEN")
	}

	if !strings.HasPrefix(botToken, "Bot ") {
		botToken = "Bot " + botToken
	}

	var store stores.StoreCloser

	switch driver := os.Getenv("STORE_DRIVER"); driver {
	case "sqlite":
		store = stores.Must(stores.NewSQLite(ctx, os.Getenv("SQLITE_URL")))
		log.Println("using SQLite")
	case "postgresql":
		store = stores.Must(stores.NewPostgreSQL(ctx, os.Getenv("POSTGRESQL_URL")))
		log.Println("using PostgreSQL")
	default:
		log.Fatalf("unknown $STORE_DRIVER %q", driver)
	}

	defer store.Close()

	opts := bot.Opts{
		Store: store,
		EmailHosts: acmregister.EmailHostsVerifier{
			"csu.fullerton.edu",
			"fullerton.edu",
		},
	}

	if shibbolethURL := os.Getenv("VERIFY_SHIBBOLETH_URL"); shibbolethURL != "" {
		opts.ShibbolethVerifier = &verifyemail.ShibbolethVerifier{
			URL: shibbolethURL,
		}
	}

	smtpInfo := verifyemail.SMTPInfo{
		Host:         os.Getenv("VERIFY_SMTP_HOST"),
		Email:        os.Getenv("VERIFY_SMTP_EMAIL"),
		Password:     os.Getenv("VERIFY_SMTP_PASSWORD"),
		TemplatePath: os.Getenv("VERIFY_SMTP_TEMPLATE_PATH"),
	}

	if smtpInfo != (verifyemail.SMTPInfo{}) {
		v, err := verifyemail.NewSMTPVerifier(smtpInfo, store)
		if err != nil {
			log.Fatalln("cannot create SMTP verifier:", err)
		}

		opts.SMTPVerifier = v
		log.Println("got SMTP credentials, enabling SMTP verification")
	}

	var start func()
	var h *bot.Handler

	if serverAddr := os.Getenv("INTERACTION_SERVER_ADDRESS"); serverAddr != "" {
		ses := state.NewAPIOnlyState(botToken, nil)

		h = bot.NewHandler(ses, opts)

		srv, err := webhook.NewInteractionServer(os.Getenv("INTERACTION_SERVER_PUBKEY"), h)
		if err != nil {
			log.Fatalln("cannot create interaction server handler:", err)
		}

		start = func() {
			log.Println("listening and serve HTTP at", serverAddr)

			if err := listener.HTTPListenAndServeCtx(ctx, &http.Server{
				Addr:    serverAddr,
				Handler: srv,
			}); err != nil {
				log.Fatalln("cannot serve HTTP:", err)
			}
		}
	} else {
		ses := state.New(botToken)
		ses.AddHandler(func(*gateway.ReadyEvent) {
			user, _ := ses.Me()
			log.Println("connected to Discord as", user.Tag())
		})
		defer ses.Close()

		h = bot.NewHandler(ses, opts)

		ses.AddIntents(h.Intents())
		ses.AddInteractionHandler(h)

		start = func() {
			log.Println("connecting to the Discord gateway...")

			if err := ses.Connect(ctx); err != nil {
				log.Fatalln("cannot connect:", err)
			}
		}
	}

	if err := h.OverwriteCommands(); err != nil {
		log.Fatalln("cannot apply commands:", err)
	}

	start()
	log.Println("shutting down...")
}
