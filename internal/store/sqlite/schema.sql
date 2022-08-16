-- NEW VERSION
-- Separate future migrations by the above comment.
PRAGMA strict = ON;

CREATE TABLE
	known_guilds (
		guild_id BIGINT PRIMARY KEY,
		channel_id BIGINT NOT NULL,
		role_id BIGINT NOT NULL,
		init_user_id BIGINT NOT NULL,
		registered_message TEXT NOT NULL
	);

CREATE TABLE
	members (
		guild_id BIGINT NOT NULL REFERENCES known_guilds(guild_id) ON DELETE CASCADE,
		user_id BIGINT NOT NULL,
		email TEXT NOT NULL,
		metadata TEXT NOT NULL,
		UNIQUE (guild_id, user_id),
		UNIQUE (guild_id, email)
	);
