CREATE TABLE IF NOT EXISTS endpoint_versions (
    id uuid PRIMARY KEY,
    endpoint_id uuid NOT NULL REFERENCES endpoints(id) ON DELETE CASCADE,
    prompt text NOT NULL,
    schema text,
    version int NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT endpoint_versions_endpoint_version_key UNIQUE (endpoint_id, version)
);

CREATE INDEX IF NOT EXISTS endpoint_versions_endpoint_id_idx ON endpoint_versions (endpoint_id);
