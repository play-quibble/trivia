-- +goose Up

-- Inline question creation in the QuizBuilder needs somewhere to store questions
-- without making the host pick a bank first. Each user gets one implicit "default"
-- bank (surfaced as e.g. "Quick questions") that inline-created questions land in.
-- questions.bank_id is NOT NULL, so this avoids a nullable-FK refactor.
ALTER TABLE question_banks
    ADD COLUMN is_default boolean NOT NULL DEFAULT false;

-- Enforce at most one default bank per owner. Partial unique index so it only
-- applies to the default rows; normal banks are unconstrained.
CREATE UNIQUE INDEX question_banks_one_default_per_owner
    ON question_banks (owner_id)
    WHERE is_default;

-- +goose Down
DROP INDEX IF EXISTS question_banks_one_default_per_owner;
ALTER TABLE question_banks DROP COLUMN IF EXISTS is_default;
