CREATE TABLE IF NOT EXISTS mock_resources (
    id uuid PRIMARY KEY,
    collection varchar(255) NOT NULL,
    resource_id varchar(255) NOT NULL,
    data jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT mock_resources_collection_resource_id_key UNIQUE (collection, resource_id)
);

CREATE INDEX IF NOT EXISTS mock_resources_collection_idx ON mock_resources (collection);
