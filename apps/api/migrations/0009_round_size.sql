-- +goose Up
ALTER TABLE games ADD COLUMN round_size integer NOT NULL DEFAULT 5;
ALTER TABLE games ADD CONSTRAINT games_round_size_positive CHECK (round_size >= 1);

-- +goose Down
ALTER TABLE games DROP COLUMN IF EXISTS round_size;
