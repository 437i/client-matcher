-- +goose Up
LOCK TABLE client_links IN ACCESS EXCLUSIVE MODE;

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM client_links
        WHERE btrim(site_id, E' \t\n\r\f\013') !~ '^[+-]?[0-9]+$'
    ) THEN
        RAISE EXCEPTION 'client_links.site_id contains invalid decimal values';
    END IF;

    BEGIN
        PERFORM btrim(site_id, E' \t\n\r\f\013')::BIGINT FROM client_links;
    EXCEPTION WHEN numeric_value_out_of_range THEN
        RAISE EXCEPTION 'client_links.site_id contains values outside BIGINT range';
    END;

    IF EXISTS (
        SELECT 1 FROM client_links
        GROUP BY btrim(site_id, E' \t\n\r\f\013')::BIGINT
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'client_links.site_id contains numeric duplicates';
    END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE client_links DROP CONSTRAINT chk_client_links_site_id_not_empty;
ALTER TABLE client_links ALTER COLUMN site_id TYPE BIGINT
    USING btrim(site_id, E' \t\n\r\f\013')::BIGINT;

-- +goose Down
LOCK TABLE client_links IN ACCESS EXCLUSIVE MODE;
ALTER TABLE client_links ALTER COLUMN site_id TYPE TEXT USING site_id::TEXT;
ALTER TABLE client_links ADD CONSTRAINT chk_client_links_site_id_not_empty
    CHECK (BTRIM(site_id) <> '');
