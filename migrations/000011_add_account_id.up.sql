-- Seed the synthetic legacy account that owns all pre-existing resources.
-- Its password is a random bcrypt hash; it is never used to log in.
INSERT INTO accounts (id, email, password_hash)
VALUES ('00000000-0000-0000-0000-000000000001', 'legacy@local', '$2a$10$IbVCEdqT0HZr6prozukTbe2QzqWxrabROUUGVwfCCF446oXrZcMqG')
ON CONFLICT (email) DO NOTHING;

ALTER TABLE endpoints
    ADD COLUMN IF NOT EXISTS account_id uuid REFERENCES accounts(id);

ALTER TABLE endpoint_versions
    ADD COLUMN IF NOT EXISTS account_id uuid REFERENCES accounts(id);

ALTER TABLE request_history
    ADD COLUMN IF NOT EXISTS account_id uuid REFERENCES accounts(id);

ALTER TABLE mock_resources
    ADD COLUMN IF NOT EXISTS account_id uuid REFERENCES accounts(id);

-- Backfill every existing row to the legacy account.
UPDATE endpoints
SET account_id = '00000000-0000-0000-0000-000000000001'
WHERE account_id IS NULL;

UPDATE endpoint_versions ev
SET account_id = '00000000-0000-0000-0000-000000000001'
FROM endpoints e
WHERE ev.endpoint_id = e.id;

UPDATE request_history rh
SET account_id = '00000000-0000-0000-0000-000000000001'
FROM endpoints e
WHERE rh.endpoint_id = e.id;

UPDATE mock_resources
SET account_id = '00000000-0000-0000-0000-000000000001'
WHERE account_id IS NULL;

CREATE INDEX IF NOT EXISTS endpoints_account_id_idx ON endpoints (account_id);
CREATE INDEX IF NOT EXISTS endpoint_versions_account_id_idx ON endpoint_versions (account_id);
CREATE INDEX IF NOT EXISTS request_history_account_id_idx ON request_history (account_id);
CREATE INDEX IF NOT EXISTS mock_resources_account_id_idx ON mock_resources (account_id);