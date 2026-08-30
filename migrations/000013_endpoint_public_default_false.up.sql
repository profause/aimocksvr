-- Mock endpoints are private by default: openness is an explicit opt-in.
-- Existing rows keep their stored value; only new inserts are affected.
ALTER TABLE endpoints
    ALTER COLUMN public SET DEFAULT false;

ALTER TABLE endpoint_versions
    ALTER COLUMN public SET DEFAULT false;