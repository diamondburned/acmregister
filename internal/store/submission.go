package store

import (
	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/arikawa/v3/discord"
)

// SubmissionStore implements a subset of Store.
type SubmissionStore struct {
	store inMemoryStore[submissionEntryKey, acmregister.MemberMetadata]
}

type submissionEntryKey struct {
	guildID discord.GuildID
	userID  discord.UserID
}

// NewSubmissionStore creates a new SubmissionStore instance.
func NewSubmissionStore() *SubmissionStore {
	var s SubmissionStore
	s.store.init(acmregister.SubmissionSaveDuration)
	s.store.startGC()
	return &s
}

func (s *SubmissionStore) Close() {
	s.store.Close()
}

func (s *SubmissionStore) SaveSubmission(gID discord.GuildID, uID discord.UserID, m acmregister.MemberMetadata) error {
	s.store.Set(submissionEntryKey{gID, uID}, m)
	return nil
}

func (s *SubmissionStore) RestoreSubmission(gID discord.GuildID, uID discord.UserID) (*acmregister.MemberMetadata, error) {
	m, ok := s.store.Get(submissionEntryKey{gID, uID})
	if ok {
		return &m, nil
	}

	return nil, acmregister.ErrNotFound
}
