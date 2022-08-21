package store

import (
	"context"
	"sync"
	"time"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/verifyemail"
	"github.com/diamondburned/arikawa/v3/discord"
)

type guildPINs struct {
	store inMemoryStore[verifyemail.PIN, acmregister.Email]
}

func newGuildPINs() *guildPINs {
	var pins guildPINs
	pins.store.init(acmregister.SubmissionSaveDuration)
	return &pins
}

// PINStore implements verifyemail.PINStore.
type PINStore struct {
	guilds map[discord.GuildID]*guildPINs
	gmut   sync.RWMutex
	gc     storeGCWorker
	ctx    context.Context
}

// NewPINStore creates a new PINStore instance.
func NewPINStore() *PINStore {
	store := PINStore{
		guilds: map[discord.GuildID]*guildPINs{},
		ctx:    context.Background(),
	}
	store.gc.Start(acmregister.SubmissionSaveDuration)
	return &store
}

func (s *PINStore) WithContext(ctx context.Context) acmregister.ContainsContext {
	cpy := *s
	cpy.ctx = ctx
	return &cpy
}

func (s *PINStore) Close() {
	s.gc.Close()

	s.gmut.Lock()
	defer s.gmut.Unlock()

	for _, guild := range s.guilds {
		guild.store.Close()
	}
}

func (s *PINStore) guild(id discord.GuildID, create bool) *guildPINs {
	s.gmut.RLock()
	pins, ok := s.guilds[id]
	s.gmut.RUnlock()
	if ok {
		return pins
	}

	if !create {
		return nil
	}

	s.gmut.Lock()
	defer s.gmut.Unlock()

	pins, ok = s.guilds[id]
	if ok {
		return pins
	}

	pins = newGuildPINs()

	s.gc.Add(pins.store.doGC)
	s.guilds[id] = pins

	return pins
}

func (s *PINStore) GeneratePIN(guildID discord.GuildID, email acmregister.Email) (verifyemail.PIN, error) {
	guild := s.guild(guildID, true)

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return verifyemail.InvalidPIN, s.ctx.Err()
		default:
			pin := verifyemail.GeneratePIN()

			_, ok := guild.store.GetOrSet(pin, email)
			if ok {
				return pin, nil
			}
		}
	}
}

func (s *PINStore) ValidatePIN(guildID discord.GuildID, pin verifyemail.PIN) (acmregister.Email, error) {
	guild := s.guild(guildID, false)
	if guild == nil {
		return "", acmregister.ErrNotFound
	}

	email, ok := guild.store.Get(pin)
	if ok {
		return email, nil
	}

	return "", acmregister.ErrNotFound
}
