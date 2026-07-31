ALTER TABLE endpoints ADD COLUMN IF NOT EXISTS request_schema text NOT NULL DEFAULT '';
