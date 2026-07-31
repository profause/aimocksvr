ALTER TABLE endpoints ADD COLUMN IF NOT EXISTS stateful boolean NOT NULL DEFAULT false;
