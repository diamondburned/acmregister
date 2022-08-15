package store

import (
	"io"
	"sync"
	"time"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/arikawa/v3/discord"
)

type StoreCloser interface {
	acmregister.Store
	io.Closer
}

// SubmissionStore implements a subset of Store.
type SubmissionStore struct {
	mut     sync.Mutex
	entries map[submissionEntryKey]submissionEntry
	stop    chan struct{}
	wg      sync.WaitGroup
}

// NewSubmissionStore creates a new SubmissionStore instance.
func NewSubmissionStore() *SubmissionStore {
	s := SubmissionStore{
		entries: make(map[submissionEntryKey]submissionEntry, 32),
		stop:    make(chan struct{}),
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.mut.Lock()
				s.gc()
				s.mut.Unlock()
			case <-s.stop:
				return
			}
		}
	}()

	return &s
}

func (s *SubmissionStore) Close() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	s.wg.Wait()
}

func (s *SubmissionStore) gc() {
	for k, entry := range s.entries {
		if entry.isExpired() {
			delete(s.entries, k)
		}
	}
}

func (s *SubmissionStore) SaveSubmission(gID discord.GuildID, uID discord.UserID, m acmregister.MemberMetadata) error {
	s.mut.Lock()
	defer s.mut.Unlock()

	k := submissionEntryKey{gID, uID}
	s.entries[k] = submissionEntry{
		MemberMetadata: m,
		Time:           time.Now(),
	}

	return nil
}

func (s *SubmissionStore) RestoreSubmission(gID discord.GuildID, uID discord.UserID) (*acmregister.MemberMetadata, error) {
	s.mut.Lock()
	defer s.mut.Unlock()

	k := submissionEntryKey{gID, uID}

	e, ok := s.entries[k]
	if ok && !e.isExpired() {
		m := e.MemberMetadata
		return &m, nil
	}

	return nil, acmregister.ErrNotFound
}

type submissionEntryKey struct {
	guildID discord.GuildID
	userID  discord.UserID
}

type submissionEntry struct {
	acmregister.MemberMetadata
	Time time.Time
}

func (e submissionEntry) isExpired() bool {
	return e.Time.Add(15 * time.Minute).Before(time.Now())
}
