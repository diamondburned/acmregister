package stores

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/diamondburned/acmregister/acmregister"
	"github.com/diamondburned/acmregister/acmregister/logger"
	"github.com/diamondburned/acmregister/acmregister/verifyemail"
	"github.com/diamondburned/acmregister/internal/stores/postgres"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
)

type pgStore struct {
	q   *postgres.Queries
	db  *pgxpool.Pool
	ctx context.Context
}

func NewPostgreSQL(ctx context.Context, uri string) (StoreCloser, error) {
	db, err := postgres.Connect(ctx, uri)
	if err != nil {
		return nil, errors.Wrap(err, "sql/pgx")
	}

	if err := db.Ping(ctx); err != nil {
		return nil, errors.Wrap(err, "sql/pgx: cannot ping pgSQL")
	}

	if err := postgres.Migrate(ctx, db); err != nil {
		return nil, errors.Wrap(err, "cannot migrate postgresql db")
	}

	s := pgStore{
		q:   postgres.New(db),
		db:  db,
		ctx: ctx,
	}

	return s, nil
}

func (s pgStore) Close() error {
	s.db.Close()
	return nil
}

func (s pgStore) WithContext(ctx context.Context) acmregister.ContainsContext {
	s.ctx = ctx
	return s
}

func (s pgStore) InitGuild(guild acmregister.KnownGuild) error {
	return s.q.InitGuild(s.ctx, postgres.InitGuildParams{
		GuildID:           int64(guild.GuildID),
		ChannelID:         int64(guild.ChannelID),
		RoleID:            int64(guild.RoleID),
		InitUserID:        int64(guild.InitUserID),
		RegisteredMessage: guild.RegisteredMessage,
	})
}

func (s pgStore) GuildInfo(guildID discord.GuildID) (*acmregister.KnownGuild, error) {
	v, err := s.q.GuildInfo(s.ctx, int64(guildID))
	if err != nil {
		return nil, postgresErr(err)
	}

	return &acmregister.KnownGuild{
		GuildID:           discord.GuildID(v.GuildID),
		ChannelID:         discord.ChannelID(v.ChannelID),
		RoleID:            discord.RoleID(v.RoleID),
		InitUserID:        discord.UserID(v.InitUserID),
		RegisteredMessage: v.RegisteredMessage,
		AdminRoleID:       discord.RoleID(v.AdminRoleID.Int64),
	}, nil
}

func (s pgStore) GuildSetAdminRole(guildID discord.GuildID, roleID discord.RoleID) error {
	n, err := s.q.SetGuildAdminRoleID(s.ctx, postgres.SetGuildAdminRoleIDParams{
		GuildID:     int64(guildID),
		AdminRoleID: pgtype.Int8{Int64: int64(roleID), Valid: roleID.IsValid()},
	})
	if err != nil {
		return postgresErr(err)
	}
	if n == 0 {
		return acmregister.ErrNotFound
	}
	return nil
}

func (s pgStore) DeleteGuild(guildID discord.GuildID) error {
	n, err := s.q.DeleteGuild(s.ctx, int64(guildID))
	if err != nil {
		return postgresErr(err)
	}
	if n == 0 {
		return acmregister.ErrNotFound
	}
	return nil
}

func (s pgStore) MemberInfo(guildID discord.GuildID, userID discord.UserID) (*acmregister.MemberMetadata, error) {
	b, err := s.q.MemberInfo(s.ctx, postgres.MemberInfoParams{
		GuildID: int64(guildID),
		UserID:  int64(userID),
	})
	if err != nil {
		return nil, postgresErr(err)
	}

	var metadata acmregister.MemberMetadata
	if err := json.Unmarshal(b, &metadata); err != nil {
		return nil, errors.Wrap(err, "member metadata JSON is corrupted")
	}

	if metadata == (acmregister.MemberMetadata{}) {
		// We used to have a bug where [acmregister.Member] was used for
		// marshaling, so we have to try and fix that.
		var memberFix struct {
			Metadata acmregister.MemberMetadata
		}
		if err := json.Unmarshal(b, &memberFix); err != nil {
			return nil, errors.Wrap(err, "member metadata JSON is corrupted")
		}
		if memberFix.Metadata == (acmregister.MemberMetadata{}) {
			return nil, errors.New("member metadata is empty")
		}
		return &memberFix.Metadata, nil
	}

	return &metadata, nil
}

func (s pgStore) RegisterMember(m acmregister.Member) error {
	pgMetadata, err := json.Marshal(m.Metadata)
	if err != nil {
		return errors.Wrap(err, "cannot encode member metadata as JSON")
	}

	tx, err := s.db.Begin(s.ctx)
	if err != nil {
		return postgresErr(err)
	}
	defer tx.Rollback(s.ctx)

	q := postgres.New(tx)

	if err := q.RegisterMember(s.ctx, postgres.RegisterMemberParams{
		GuildID:  int64(m.GuildID),
		UserID:   int64(m.UserID),
		Email:    string(m.Metadata.Email),
		Metadata: pgMetadata,
	}); err != nil {
		if postgres.IsConstraintFailed(err) {
			return acmregister.ErrMemberAlreadyExists
		}
		return postgresErr(err)
	}

	q.DeleteSubmission(s.ctx, postgres.DeleteSubmissionParams{
		GuildID: int64(m.GuildID),
		UserID:  int64(m.UserID),
	})

	if err := tx.Commit(s.ctx); err != nil {
		return postgresErr(err)
	}

	return nil
}

func (s pgStore) UnregisterMember(guildID discord.GuildID, userID discord.UserID) error {
	n, err := s.q.UnregisterMember(s.ctx, postgres.UnregisterMemberParams{
		GuildID: int64(guildID),
		UserID:  int64(userID),
	})
	if err != nil {
		return postgresErr(err)
	}
	if n == 0 {
		return acmregister.ErrNotFound
	}
	return nil
}

func (s pgStore) SaveSubmission(m acmregister.Member) error {
	pgMetadata, err := json.Marshal(m.Metadata)
	if err != nil {
		return errors.Wrap(err, "cannot encode member metadata as JSON")
	}

	err = s.q.SaveSubmission(s.ctx, postgres.SaveSubmissionParams{
		GuildID:  int64(m.GuildID),
		UserID:   int64(m.UserID),
		Metadata: pgMetadata,
	})
	if err != nil {
		return postgresErr(err)
	}

	s.q.CleanupSubmissions(s.ctx)
	return nil
}

func (s pgStore) RestoreSubmission(guildID discord.GuildID, userID discord.UserID) (*acmregister.MemberMetadata, error) {
	b, err := s.q.RestoreSubmission(s.ctx, postgres.RestoreSubmissionParams{
		GuildID: int64(guildID),
		UserID:  int64(userID),
	})
	if err != nil {
		return nil, postgresErr(err)
	}

	var metadata acmregister.MemberMetadata
	if err := json.Unmarshal(b, &metadata); err != nil {
		return nil, errors.Wrap(err, "member metadata JSON is corrupted")
	}

	return &metadata, nil
}

func (s pgStore) GeneratePIN(guildID discord.GuildID, userID discord.UserID) (verifyemail.PIN, error) {
	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	defer cancel()

	log := logger.FromContext(ctx)

	for {
		select {
		case <-ctx.Done():
			return verifyemail.InvalidPIN, ctx.Err()
		default:
		}

		pin := verifyemail.GeneratePIN()
		log.Println("PIN generated, inserting...")
		err := s.q.InsertPIN(ctx, postgres.InsertPINParams{
			GuildID: int64(guildID),
			UserID:  int64(userID),
			Pin:     int16(pin),
		})
		if err == nil {
			log.Println("PIN inserted OK")
			return pin, nil
		}
		if postgres.IsConstraintFailed(err) {
			log.Println("same PIN is already used for user and guild, retrying")
			continue
		}
		log.Println("PIN store error:", err)
		return verifyemail.InvalidPIN, errors.Wrap(err, "cannot store PIN")
	}
}

func (s pgStore) ValidatePIN(guildID discord.GuildID, userID discord.UserID, pin verifyemail.PIN) (*acmregister.MemberMetadata, error) {
	b, err := s.q.ValidatePIN(s.ctx, postgres.ValidatePINParams{
		GuildID: int64(guildID),
		UserID:  int64(userID),
		Pin:     int16(pin),
	})
	if err != nil {
		return nil, postgresErr(err)
	}

	var metadata acmregister.MemberMetadata
	if err := json.Unmarshal(b, &metadata); err != nil {
		return nil, errors.Wrap(err, "member metadata JSON is corrupted")
	}

	return &metadata, nil
}

func postgresErr(err error) error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return acmregister.ErrNotFound
	default:
		return err
	}
}
