-- +goose Up
ALTER TABLE client_links
    ADD COLUMN manually_confirmed BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN confirmation_reason TEXT,
    ADD COLUMN confirmation_codes TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN confirmed_at TIMESTAMPTZ,
    ADD COLUMN assessment_rule_version INTEGER;

ALTER TABLE client_links
    ADD CONSTRAINT chk_client_links_confirmation_consistency CHECK (
        (manually_confirmed = FALSE AND confirmation_reason IS NULL AND confirmed_at IS NULL AND cardinality(confirmation_codes) = 0)
        OR
        (manually_confirmed = TRUE AND confirmation_reason IS NOT NULL AND char_length(btrim(confirmation_reason)) BETWEEN 1 AND 1000 AND confirmed_at IS NOT NULL AND assessment_rule_version IS NOT NULL AND assessment_rule_version > 0 AND cardinality(confirmation_codes) > 0)
    );

ALTER TABLE client_links
    ADD CONSTRAINT chk_client_links_confirmation_codes CHECK (
        confirmation_codes <@ ARRAY['name:alias_match','name:probable_match','name:mismatch','name:missing','birthday:mismatch','birthday:missing']::TEXT[]
    );

-- +goose Down
ALTER TABLE client_links DROP CONSTRAINT IF EXISTS chk_client_links_confirmation_codes;
ALTER TABLE client_links DROP CONSTRAINT IF EXISTS chk_client_links_confirmation_consistency;
ALTER TABLE client_links
    DROP COLUMN IF EXISTS assessment_rule_version,
    DROP COLUMN IF EXISTS confirmed_at,
    DROP COLUMN IF EXISTS confirmation_codes,
    DROP COLUMN IF EXISTS confirmation_reason,
    DROP COLUMN IF EXISTS manually_confirmed;
