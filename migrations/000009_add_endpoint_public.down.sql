ALTER TABLE endpoint_versions
    DROP COLUMN IF EXISTS public;

ALTER TABLE endpoints
    DROP COLUMN IF EXISTS public;
