ALTER TABLE endpoint_versions
    DROP COLUMN IF EXISTS error_sim,
    DROP COLUMN IF EXISTS request_schema,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS stateful,
    DROP COLUMN IF EXISTS response_type,
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS path,
    DROP COLUMN IF EXISTS method;
