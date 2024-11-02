// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.23.0
// source: queries.sql

package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const cleanupSubmissions = `-- name: CleanupSubmissions :exec
DELETE FROM
	registration_submissions
WHERE
	expire_at < NOW()
`

func (q *Queries) CleanupSubmissions(ctx context.Context) error {
	_, err := q.db.Exec(ctx, cleanupSubmissions)
	return err
}

const deleteGuild = `-- name: DeleteGuild :execrows
DELETE FROM
	known_guilds
WHERE
	guild_id = $1
`

func (q *Queries) DeleteGuild(ctx context.Context, guildID int64) (int64, error) {
	result, err := q.db.Exec(ctx, deleteGuild, guildID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

const deleteSubmission = `-- name: DeleteSubmission :exec
DELETE FROM
	registration_submissions
WHERE
	guild_id = $1
	AND user_id = $2
`

type DeleteSubmissionParams struct {
	GuildID int64
	UserID  int64
}

func (q *Queries) DeleteSubmission(ctx context.Context, arg DeleteSubmissionParams) error {
	_, err := q.db.Exec(ctx, deleteSubmission, arg.GuildID, arg.UserID)
	return err
}

const guildInfo = `-- name: GuildInfo :one
SELECT
	guild_id, channel_id, role_id, init_user_id, registered_message, admin_role_id
FROM
	known_guilds
WHERE
	guild_id = $1
LIMIT
	1
`

func (q *Queries) GuildInfo(ctx context.Context, guildID int64) (KnownGuild, error) {
	row := q.db.QueryRow(ctx, guildInfo, guildID)
	var i KnownGuild
	err := row.Scan(
		&i.GuildID,
		&i.ChannelID,
		&i.RoleID,
		&i.InitUserID,
		&i.RegisteredMessage,
		&i.AdminRoleID,
	)
	return i, err
}

const initGuild = `-- name: InitGuild :exec
INSERT INTO
	known_guilds (
		guild_id,
		channel_id,
		init_user_id,
		role_id,
		registered_message
	)
VALUES
	($1, $2, $3, $4, $5)
`

type InitGuildParams struct {
	GuildID           int64
	ChannelID         int64
	InitUserID        int64
	RoleID            int64
	RegisteredMessage string
}

func (q *Queries) InitGuild(ctx context.Context, arg InitGuildParams) error {
	_, err := q.db.Exec(ctx, initGuild,
		arg.GuildID,
		arg.ChannelID,
		arg.InitUserID,
		arg.RoleID,
		arg.RegisteredMessage,
	)
	return err
}

const insertPIN = `-- name: InsertPIN :exec
INSERT INTO
	pin_codes (guild_id, user_id, pin)
VALUES
	($1, $2, $3) ON CONFLICT (guild_id, user_id)
DO
UPDATE
SET
	pin = $3
`

type InsertPINParams struct {
	GuildID int64
	UserID  int64
	Pin     int16
}

func (q *Queries) InsertPIN(ctx context.Context, arg InsertPINParams) error {
	_, err := q.db.Exec(ctx, insertPIN, arg.GuildID, arg.UserID, arg.Pin)
	return err
}

const memberInfo = `-- name: MemberInfo :one
SELECT
	metadata
FROM
	members
WHERE
	guild_id = $1
	AND user_id = $2
`

type MemberInfoParams struct {
	GuildID int64
	UserID  int64
}

func (q *Queries) MemberInfo(ctx context.Context, arg MemberInfoParams) ([]byte, error) {
	row := q.db.QueryRow(ctx, memberInfo, arg.GuildID, arg.UserID)
	var metadata []byte
	err := row.Scan(&metadata)
	return metadata, err
}

const registerMember = `-- name: RegisterMember :exec
INSERT INTO
	members (guild_id, user_id, email, metadata)
VALUES
	($1, $2, $3, $4)
`

type RegisterMemberParams struct {
	GuildID  int64
	UserID   int64
	Email    string
	Metadata []byte
}

func (q *Queries) RegisterMember(ctx context.Context, arg RegisterMemberParams) error {
	_, err := q.db.Exec(ctx, registerMember,
		arg.GuildID,
		arg.UserID,
		arg.Email,
		arg.Metadata,
	)
	return err
}

const restoreSubmission = `-- name: RestoreSubmission :one
SELECT
	metadata
FROM
	registration_submissions
WHERE
	guild_id = $1
	AND user_id = $2
	AND expire_at >= NOW()
`

type RestoreSubmissionParams struct {
	GuildID int64
	UserID  int64
}

func (q *Queries) RestoreSubmission(ctx context.Context, arg RestoreSubmissionParams) ([]byte, error) {
	row := q.db.QueryRow(ctx, restoreSubmission, arg.GuildID, arg.UserID)
	var metadata []byte
	err := row.Scan(&metadata)
	return metadata, err
}

const saveSubmission = `-- name: SaveSubmission :exec
INSERT INTO
	registration_submissions (guild_id, user_id, metadata, expire_at)
VALUES
	($1, $2, $3, NOW() + INTERVAL '1 hour') ON CONFLICT (guild_id, user_id)
DO
UPDATE
SET
	metadata = EXCLUDED.metadata,
	expire_at = EXCLUDED.expire_at
`

type SaveSubmissionParams struct {
	GuildID  int64
	UserID   int64
	Metadata []byte
}

func (q *Queries) SaveSubmission(ctx context.Context, arg SaveSubmissionParams) error {
	_, err := q.db.Exec(ctx, saveSubmission, arg.GuildID, arg.UserID, arg.Metadata)
	return err
}

const setGuildAdminRoleID = `-- name: SetGuildAdminRoleID :execrows
UPDATE
	known_guilds
SET
	admin_role_id = $2
WHERE
	guild_id = $1
`

type SetGuildAdminRoleIDParams struct {
	GuildID     int64
	AdminRoleID pgtype.Int8
}

func (q *Queries) SetGuildAdminRoleID(ctx context.Context, arg SetGuildAdminRoleIDParams) (int64, error) {
	result, err := q.db.Exec(ctx, setGuildAdminRoleID, arg.GuildID, arg.AdminRoleID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

const unregisterMember = `-- name: UnregisterMember :execrows
DELETE FROM
	members
WHERE
	guild_id = $1
	AND user_id = $2
`

type UnregisterMemberParams struct {
	GuildID int64
	UserID  int64
}

func (q *Queries) UnregisterMember(ctx context.Context, arg UnregisterMemberParams) (int64, error) {
	result, err := q.db.Exec(ctx, unregisterMember, arg.GuildID, arg.UserID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

const validatePIN = `-- name: ValidatePIN :one
SELECT
	registration_submissions.metadata
FROM
	registration_submissions
	JOIN pin_codes ON registration_submissions.guild_id = pin_codes.guild_id
	AND registration_submissions.user_id = pin_codes.user_id
WHERE
	pin_codes.guild_id = $1
	AND pin_codes.user_id = $2
	AND pin_codes.pin = $3
	AND registration_submissions.expire_at >= NOW()
`

type ValidatePINParams struct {
	GuildID int64
	UserID  int64
	Pin     int16
}

func (q *Queries) ValidatePIN(ctx context.Context, arg ValidatePINParams) ([]byte, error) {
	row := q.db.QueryRow(ctx, validatePIN, arg.GuildID, arg.UserID, arg.Pin)
	var metadata []byte
	err := row.Scan(&metadata)
	return metadata, err
}

const version = `-- name: Version :one
SELECT
	v
FROM
	meta
`

// Language: postgresql
func (q *Queries) Version(ctx context.Context) (int16, error) {
	row := q.db.QueryRow(ctx, version)
	var v int16
	err := row.Scan(&v)
	return v, err
}
