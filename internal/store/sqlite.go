package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/internal/store/sqlite"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/pkg/errors"
)

type sqliteStore struct {
	*SubmissionStore
	*PINStore

	q   *sqlite.Queries
	db  *sql.DB
	ctx context.Context
}

func NewSQLite(ctx context.Context, uri string) (StoreCloser, error) {
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return nil, errors.Wrap(err, "sql/sqlite3")
	}

	if err := sqlite.Migrate(ctx, db); err != nil {
		return nil, errors.Wrap(err, "cannot migrate sqlite db")
	}

	s := sqliteStore{
		SubmissionStore: NewSubmissionStore(),
		PINStore:        NewPINStore(),

		q:   sqlite.New(db),
		db:  db,
		ctx: ctx,
	}

	return s, nil
}

func (s sqliteStore) Close() error {
	s.SubmissionStore.Close()
	err := s.db.Close()
	return err
}

func (s sqliteStore) WithContext(ctx context.Context) acmregister.ContainsContext {
	s.PINStore = s.PINStore.WithContext(ctx).(*PINStore)
	s.ctx = ctx
	return s
}

func (s sqliteStore) InitGuild(guild acmregister.KnownGuild) error {
	return s.q.InitGuild(s.ctx, sqlite.InitGuildParams{
		GuildID:           int64(guild.GuildID),
		ChannelID:         int64(guild.ChannelID),
		RoleID:            int64(guild.RoleID),
		InitUserID:        int64(guild.InitUserID),
		RegisteredMessage: guild.RegisteredMessage,
	})
}

func (s sqliteStore) GuildInfo(guildID discord.GuildID) (*acmregister.KnownGuild, error) {
	v, err := s.q.GuildInfo(s.ctx, int64(guildID))
	if err != nil {
		return nil, sqlErr(err)
	}

	return &acmregister.KnownGuild{
		GuildID:           discord.GuildID(v.GuildID),
		ChannelID:         discord.ChannelID(v.ChannelID),
		RoleID:            discord.RoleID(v.RoleID),
		InitUserID:        discord.UserID(v.InitUserID),
		RegisteredMessage: v.RegisteredMessage,
	}, nil
}

func (s sqliteStore) DeleteGuild(guildID discord.GuildID) error {
	n, err := s.q.DeleteGuild(s.ctx, int64(guildID))
	if err != nil {
		return sqlErr(err)
	}
	if n == 0 {
		return acmregister.ErrNotFound
	}
	return nil
}

func (s sqliteStore) MemberInfo(guildID discord.GuildID, userID discord.UserID) (*acmregister.MemberMetadata, error) {
	v, err := s.q.MemberInfo(s.ctx, sqlite.MemberInfoParams{
		GuildID: int64(guildID),
		UserID:  int64(userID),
	})
	if err != nil {
		return nil, sqlErr(err)
	}

	var metadata acmregister.MemberMetadata

	if err := json.Unmarshal([]byte(v.Metadata), &metadata); err != nil {
		return nil, errors.Wrap(err, "member metadata JSON is corrupted")
	}

	return &metadata, nil
}

func (s sqliteStore) RegisterMember(guildID discord.GuildID, userID discord.UserID, m acmregister.MemberMetadata) error {
	b, err := json.Marshal(m)
	if err != nil {
		return errors.Wrap(err, "cannot encode member metadata as JSON")
	}

	if err := s.q.RegisterMember(s.ctx, sqlite.RegisterMemberParams{
		GuildID:  int64(guildID),
		UserID:   int64(userID),
		Email:    string(m.Email),
		Metadata: string(b),
	}); err != nil {
		if sqlite.IsConstraintFailed(err) {
			return acmregister.ErrMemberAlreadyExists
		}
		return sqlErr(err)
	}

	return nil
}

func (s sqliteStore) UnregisterMember(guildID discord.GuildID, userID discord.UserID) error {
	n, err := s.q.UnregisterMember(s.ctx, sqlite.UnregisterMemberParams{
		GuildID: int64(guildID),
		UserID:  int64(userID),
	})
	if err != nil {
		return sqlErr(err)
	}
	if n == 0 {
		return acmregister.ErrNotFound
	}
	return nil
}

func sqlErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return acmregister.ErrNotFound
	}
	return err
}
