-- +goose Up
-- +goose StatementBegin
-- Add Prism model columns to messages table. Set on Hyper turns served
-- through a Prism model, from the X-Prism-* response headers and trailers.
ALTER TABLE messages ADD COLUMN prism_model_id TEXT;
ALTER TABLE messages ADD COLUMN prism_model_name TEXT;
ALTER TABLE messages ADD COLUMN prism_hypercredit_savings REAL;
ALTER TABLE messages ADD COLUMN prism_dollar_savings REAL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Remove Prism model columns from messages table
ALTER TABLE messages DROP COLUMN prism_model_id;
ALTER TABLE messages DROP COLUMN prism_model_name;
ALTER TABLE messages DROP COLUMN prism_hypercredit_savings;
ALTER TABLE messages DROP COLUMN prism_dollar_savings;
-- +goose StatementEnd
