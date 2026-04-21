// Package bootstrap seeds a fresh database with a system user plus a set of
// sample question banks and questions. It is idempotent: rerunning leaves the
// existing rows untouched via ON CONFLICT (id) DO NOTHING, and stable UUIDs
// are derived from slugs so the same data always maps to the same rows.
package bootstrap

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"
)

//go:embed data/seed.yaml
var seedYAML []byte

const (
	systemAuth0Sub     = "system:bootstrap"
	systemDisplayName  = "System"
	defaultQuestionPts = 1000
)

type seedFile struct {
	Banks []seedBank `yaml:"banks"`
}

type seedBank struct {
	Slug        string         `yaml:"slug"`
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Questions   []seedQuestion `yaml:"questions"`
}

type seedQuestion struct {
	Type          string       `yaml:"type"`
	Prompt        string       `yaml:"prompt"`
	CorrectAnswer string       `yaml:"correct_answer"`
	Choices       []seedChoice `yaml:"choices"`
}

type seedChoice struct {
	Text    string `yaml:"text"`
	Correct bool   `yaml:"correct"`
}

// Run seeds the database if the sample data is not already present.
// Every insert is guarded by ON CONFLICT so repeated runs are safe.
func Run(ctx context.Context, pool *pgxpool.Pool) error {
	var data seedFile
	if err := yaml.Unmarshal(seedYAML, &data); err != nil {
		return fmt.Errorf("parse seed yaml: %w", err)
	}

	systemUserID := stableUUID("user:" + systemAuth0Sub)
	if err := upsertSystemUser(ctx, pool, systemUserID); err != nil {
		return fmt.Errorf("upsert system user: %w", err)
	}

	for _, bank := range data.Banks {
		bankID := stableUUID("bank:" + bank.Slug)
		if err := upsertBank(ctx, pool, bankID, systemUserID, bank); err != nil {
			return fmt.Errorf("upsert bank %q: %w", bank.Slug, err)
		}
		for i, q := range bank.Questions {
			qID := stableUUID("question:" + bank.Slug + ":" + q.Prompt)
			if err := upsertQuestion(ctx, pool, qID, bankID, i, q); err != nil {
				return fmt.Errorf("upsert question %q in %q: %w", q.Prompt, bank.Slug, err)
			}
		}
		slog.Info("bootstrap bank ready", "slug", bank.Slug, "questions", len(bank.Questions))
	}
	return nil
}

func upsertSystemUser(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, auth0_sub, display_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (auth0_sub) DO NOTHING
	`, id, systemAuth0Sub, systemDisplayName)
	return err
}

func upsertBank(ctx context.Context, pool *pgxpool.Pool, id, ownerID uuid.UUID, bank seedBank) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO question_banks (id, owner_id, name, description)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING
	`, id, ownerID, bank.Name, bank.Description)
	return err
}

func upsertQuestion(ctx context.Context, pool *pgxpool.Pool, id, bankID uuid.UUID, position int, q seedQuestion) error {
	correct := q.CorrectAnswer
	var choicesJSON []byte
	if q.Type == "multiple_choice" {
		if len(q.Choices) < 2 {
			return fmt.Errorf("multiple_choice question needs >= 2 choices")
		}
		for _, c := range q.Choices {
			if c.Correct {
				correct = c.Text
				break
			}
		}
		if correct == "" {
			return fmt.Errorf("multiple_choice question has no choice marked correct")
		}
		b, err := json.Marshal(q.Choices)
		if err != nil {
			return fmt.Errorf("marshal choices: %w", err)
		}
		choicesJSON = b
	}
	if correct == "" {
		return fmt.Errorf("question missing correct_answer")
	}

	_, err := pool.Exec(ctx, `
		INSERT INTO questions (id, bank_id, type, prompt, correct_answer, choices, points, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO NOTHING
	`, id, bankID, q.Type, q.Prompt, correct, choicesJSON, defaultQuestionPts, position)
	return err
}

func stableUUID(key string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("quibble:"+key))
}
