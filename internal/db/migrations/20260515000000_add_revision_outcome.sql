-- +goose Up
ALTER TABLE critic_reviews ADD COLUMN revision_outcome TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE critic_reviews DROP COLUMN revision_outcome;
