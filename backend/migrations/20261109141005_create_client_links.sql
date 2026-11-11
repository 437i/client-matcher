-- +goose Up
CREATE TABLE client_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    crm_id TEXT NOT NULL,
    site_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_client_links_crm_id UNIQUE (crm_id),
    CONSTRAINT uq_client_links_site_id UNIQUE (site_id),

    CONSTRAINT chk_client_links_crm_id_not_empty
        CHECK (BTRIM(crm_id) <> ''),

    CONSTRAINT chk_client_links_site_id_not_empty
        CHECK (BTRIM(site_id) <> '')
);

-- +goose Down
DROP TABLE IF EXISTS client_links;