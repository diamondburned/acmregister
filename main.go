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
	"github.com/diamondburned/acmregister/internal/store"
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

	sqliteURI := os.Getenv("SQLITE_URI")
	if sqliteURI == "" {
		sqliteURI = "file::memory:"
		log.Println("Missing $SQLITE_URI, using in-memory database instead (will be wiped after shutdown)")
	}

	ses := state.New("Bot " + botToken)
	ses.AddHandler(func(*gateway.ReadyEvent) {
		user, _ := ses.Me()
		log.Println("Connected to Discord as", user.Tag())
	})

	store, err := store.NewSQLite(ctx, sqliteURI)
	if err != nil {
		log.Fatalln("cannot create SQLite db:", err)
	}
	defer store.Close()

	opts := bot.Opts{
		Store: store,
		EmailHosts: acmregister.EmailHostsVerifier{
			"csu.fullerton.edu",
			"fullerton.edu",
		},
		ShibbolethVerifier: &verifyemail.ShibbolethVerifier{
			URL: os.Getenv("VERIFY_SHIBBOLETH_URL"),
		},
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
