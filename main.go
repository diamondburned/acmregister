package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/internal/store"
	"github.com/diamondburned/arikawa/v3/state"
)

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

	store, err := store.NewSQLite(ctx, sqliteURI)
	if err != nil {
		log.Fatalln("cannot create SQLite db:", err)
	}
	defer store.Close()

	log.Println("Binding commands...")
	if err := acmregister.Bind(ses, store); err != nil {
		log.Fatalln("cannot bind acmregister:", err)
	}

	log.Println("Commands bound, starting gateway...")
	if err := ses.Connect(ctx); err != nil {
		log.Fatalln("cannot connect:", err)
	}
}
