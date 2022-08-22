package main

import (
	"context"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/bot"
	"github.com/diamondburned/acmregister/acmregister/verifyemail"
	"github.com/diamondburned/acmregister/internal/stores"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
)

func init() { rand.Seed(time.Now().UnixNano()) }

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatalln("no $BOT_TOKEN")
	}

	ses := state.New("Bot " + botToken)
	ses.AddHandler(func(*gateway.ReadyEvent) {
		user, _ := ses.Me()
		log.Println("Connected to Discord as", user.Tag())
	})

	var store interface {
		acmregister.Store
		verifyemail.PINStore
	}

	switch driver := os.Getenv("STORE_DRIVER"); driver {
	case "sqlite":
		s := stores.Must(stores.NewSQLite(ctx, os.Getenv("SQLITE_URL")))
		defer s.Close()
		store = s
	case "postgresql":
		s := stores.Must(stores.NewPostgreSQL(ctx, os.Getenv("POSTGRESQL_URL")))
		defer s.Close()
		store = s
	default:
		log.Fatalf("unknown $STORE_DRIVER %q", driver)
	}

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
			log.Fatalln("cannot create SMTP verifier")
		}

		opts.SMTPVerifier = v
		log.Println("Got SMTP credentials, enabling SMTP verification")
	}

	log.Println("Binding commands...")
	if err := bot.Bind(ses, opts); err != nil {
		log.Fatalln("cannot bind acmregister:", err)
	}

	log.Println("Commands bound, starting gateway...")
	if err := ses.Connect(ctx); err != nil {
		log.Fatalln("cannot connect:", err)
	}

	log.Println("Shutting down...")
}
