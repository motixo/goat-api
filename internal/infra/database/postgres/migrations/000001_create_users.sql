CREATE TABLE users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    status SMALLINT NOT NULL,
    role SMALLINT NOT NULL,
    credential_version BIGINT NOT NULL DEFAULT 1 CHECK (credential_version > 0),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NULL
);
