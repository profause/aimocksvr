ALTER TABLE endpoints
    ALTER COLUMN public SET DEFAULT true;

ALTER TABLE endpoint_versions
    ALTER COLUMN public SET DEFAULT true;