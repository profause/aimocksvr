ALTER TABLE endpoint_versions
    ADD COLUMN IF NOT EXISTS method varchar(10) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS path varchar(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS response_type varchar(50) NOT NULL DEFAULT 'json',
    ADD COLUMN IF NOT EXISTS stateful boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS status varchar(20) NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS request_schema text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS error_sim text NOT NULL DEFAULT '';

-- Backfill legacy snapshots (which only stored prompt + schema) with the
-- endpoint's current state so historical versions remain rollback-able.
UPDATE endpoint_versions ev
SET method        = e.method,
    path          = e.path,
    description   = e.description,
    response_type = e.response_type,
    stateful      = e.stateful,
    status        = e.status,
    request_schema = e.request_schema,
    error_sim     = e.error_sim
FROM endpoints e
WHERE ev.endpoint_id = e.id
  AND ev.method = '';
