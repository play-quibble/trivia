-- +goose Up

-- A Quiz is a host-curated set of rounds, each containing manually-selected questions.
-- This replaces the old "bank → game" model where every question in a bank was used
-- in the order it was stored.
CREATE TABLE quizzes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Each round within a quiz.  round_number is 1-indexed and unique per quiz.
CREATE TABLE quiz_rounds (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quiz_id      UUID NOT NULL REFERENCES quizzes(id) ON DELETE CASCADE,
    round_number INTEGER NOT NULL CHECK (round_number >= 1),
    title        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (quiz_id, round_number)
);

-- The ordered list of questions in a round.
-- Questions can come from any bank owned by the quiz owner.
-- position is 0-indexed and unique per round.
CREATE TABLE quiz_round_questions (
    round_id    UUID    NOT NULL REFERENCES quiz_rounds(id) ON DELETE CASCADE,
    question_id UUID    NOT NULL REFERENCES questions(id)   ON DELETE CASCADE,
    position    INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY (round_id, question_id),
    UNIQUE (round_id, position)
);

-- +goose Down
DROP TABLE IF EXISTS quiz_round_questions;
DROP TABLE IF EXISTS quiz_rounds;
DROP TABLE IF EXISTS quizzes;
