CREATE TABLE channel_moderator (
    id SERIAL PRIMARY KEY,
    channel_name TEXT NOT NULL,
    user_id TEXT NOT NULL
)