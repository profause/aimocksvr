CREATE TABLE IF NOT EXISTS request_history (
    id uuid PRIMARY KEY,
    endpoint_id uuid NOT NULL REFERENCES endpoints(id) ON DELETE CASCADE,
    request text,
    response text,
    latency bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS request_history_endpoint_id_idx ON request_history (endpoint_id);
CREATE INDEX IF NOT EXISTS request_history_created_at_idx ON request_history (created_at DESC);
