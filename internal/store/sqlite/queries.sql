-- name: GuildInfo :one
SELECT
	*
FROM
	known_guilds
WHERE
	guild_id = ?
LIMIT
	1;

-- name: DeleteGuild :execrows
DELETE FROM
	known_guilds
WHERE
	guild_id = ?;

-- name: InitGuild :exec
INSERT INTO
	known_guilds (
		guild_id,
		channel_id,
		init_user_id,
		role_id,
		registered_message
	)
VALUES
	(?, ?, ?, ?, ?);

-- name: RegisterMember :exec
INSERT INTO
	members (guild_id, user_id, email, metadata)
VALUES
	(?, ?, ?, ?);

-- name: UnregisterMember :execrows
DELETE FROM
	members
WHERE
	guild_id = ?
	AND user_id = ?;

-- name: MemberInfo :one
SELECT
	*
FROM
	members
WHERE
	guild_id = ?
	AND user_id = ?;
