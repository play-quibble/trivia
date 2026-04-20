-- +goose Up

-- The original quiz_id FK used ON DELETE SET NULL, which conflicts with the
-- games_bank_or_quiz CHECK constraint: if a quiz-only game has its quiz_id
-- nulled out, both bank_id and quiz_id become NULL, violating the check.
-- RESTRICT is the correct behavior — block quiz deletion if games reference it.
ALTER TABLE games DROP CONSTRAINT IF EXISTS games_quiz_id_fkey;
ALTER TABLE games
    ADD CONSTRAINT games_quiz_id_fkey
        FOREIGN KEY (quiz_id) REFERENCES quizzes(id) ON DELETE RESTRICT;

-- +goose Down

ALTER TABLE games DROP CONSTRAINT IF EXISTS games_quiz_id_fkey;
ALTER TABLE games
    ADD CONSTRAINT games_quiz_id_fkey
        FOREIGN KEY (quiz_id) REFERENCES quizzes(id) ON DELETE SET NULL;
