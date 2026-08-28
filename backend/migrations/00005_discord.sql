-- +goose Up
ALTER TABLE settings RENAME COLUMN telegram_chat_id TO discord_channel_id;

-- +goose Down
ALTER TABLE settings RENAME COLUMN discord_channel_id TO telegram_chat_id;
