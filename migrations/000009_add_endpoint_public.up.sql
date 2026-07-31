ALTER TABLE endpoints
    ADD COLUMN IF NOT EXISTS public boolean NOT NULL DEFAULT true;

ALTER TABLE endpoint_versions
    ADD COLUMN IF NOT EXISTS public boolean NOT NULL DEFAULT true;

-- Backfill legacy version snapshots from their endpoint's current state.
UPDATE endpoint_versions ev
SET public = e.public
FROM endpoints e
WHERE ev.endpoint_id = e.id;
