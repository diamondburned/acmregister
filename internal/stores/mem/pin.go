package mem

import (
	"context"
	"sync"
	"time"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/verifyemail"
	"github.com/diamondburned/arikawa/v3/discord"
)

type guildPINs struct {
	store inMemoryStore[verifyemail.PIN, discord.UserID]
}

func newGuildPINs() *guildPINs {
	var pins guildPINs
	pins.store.init(acmregister.SubmissionSaveDuration)
	return &pins
}

// PINStore implements verifyemail.PINStore.
type PINStore struct {
	*pinStore
	sub acmregister.SubmissionStore
	ctx context.Context
}

type pinStore struct {
	guilds map[discord.GuildID]*guildPINs
	gmut   sync.RWMutex
	gc     storeGCWorker
}

// NewPINStore creates a new PINStore instance.
func NewPINStore(submissions acmregister.SubmissionStore) *PINStore {
	store := PINStore{
		pinStore: &pinStore{
			guilds: map[discord.GuildID]*guildPINs{},
		},
		sub: submissions,
		ctx: context.Background(),
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

func (s *PINStore) GeneratePIN(guildID discord.GuildID, userID discord.UserID) (verifyemail.PIN, error) {
	guild := s.guild(guildID, true)

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return verifyemail.InvalidPIN, s.ctx.Err()
		default:
			pin := verifyemail.GeneratePIN()

			_, ok := guild.store.GetOrSet(pin, userID)
			if ok {
				return pin, nil
			}
		}
	}
}

func (s *PINStore) ValidatePIN(guildID discord.GuildID, pin verifyemail.PIN) (*acmregister.MemberMetadata, error) {
	guild := s.guild(guildID, false)
	if guild == nil {
		return nil, acmregister.ErrNotFound
	}

	uID, ok := guild.store.Get(pin)
	if !ok {
		return nil, acmregister.ErrNotFound
	}

	return s.sub.RestoreSubmission(guildID, uID)
}
