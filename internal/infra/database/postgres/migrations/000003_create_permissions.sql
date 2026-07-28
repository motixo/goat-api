CREATE TABLE permissions (
    id UUID PRIMARY KEY,
    role SMALLINT NOT NULL,
    action TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NULL,
    CONSTRAINT unique_role_action UNIQUE(role, action)
);
