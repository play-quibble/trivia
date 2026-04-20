-- +goose Up

-- Make bank_id nullable so quiz-based games don't need a bank.
-- Drop the existing FK constraint, then re-add it as nullable.
ALTER TABLE games
    DROP CONSTRAINT IF EXISTS games_bank_id_fkey;

-- Allow bank_id to be NULL for quiz-driven games.
ALTER TABLE games
    ALTER COLUMN bank_id DROP NOT NULL;

-- Re-add the FK as optional.
ALTER TABLE games
    ADD CONSTRAINT games_bank_id_fkey
        FOREIGN KEY (bank_id) REFERENCES question_banks(id) ON DELETE CASCADE;

-- Add quiz_id: links a game to a Quiz instead of a raw bank.
ALTER TABLE games
    ADD COLUMN quiz_id UUID REFERENCES quizzes(id) ON DELETE SET NULL;

-- Track which round we're currently on (for quiz-based games).
ALTER TABLE games
    ADD COLUMN current_round_idx INTEGER NOT NULL DEFAULT 0;

-- At least one of bank_id or quiz_id must be set.
ALTER TABLE games
    ADD CONSTRAINT games_bank_or_quiz CHECK (
        bank_id IS NOT NULL OR quiz_id IS NOT NULL
    );

-- +goose Down
ALTER TABLE games
    DROP CONSTRAINT IF EXISTS games_bank_or_quiz,
    DROP COLUMN IF EXISTS quiz_id,
    DROP COLUMN IF EXISTS current_round_idx,
    DROP CONSTRAINT IF EXISTS games_bank_id_fkey;

ALTER TABLE games
    ALTER COLUMN bank_id SET NOT NULL;

ALTER TABLE games
    ADD CONSTRAINT games_bank_id_fkey
        FOREIGN KEY (bank_id) REFERENCES question_banks(id) ON DELETE CASCADE;
