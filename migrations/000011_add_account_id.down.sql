ALTER TABLE endpoints
    DROP COLUMN IF EXISTS account_id;

ALTER TABLE endpoint_versions
    DROP COLUMN IF EXISTS account_id;

ALTER TABLE request_history
    DROP COLUMN IF EXISTS account_id;

ALTER TABLE mock_resources
    DROP COLUMN IF EXISTS account_id;