CREATE TABLE IF NOT EXISTS endpoints (
    id uuid PRIMARY KEY,
    method varchar(10) NOT NULL,
    path varchar(255) NOT NULL,
    description text NOT NULL DEFAULT '',
    prompt text NOT NULL,
    response_type varchar(50) NOT NULL DEFAULT 'json',
    status varchar(20) NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT endpoints_method_path_key UNIQUE (method, path)
);
