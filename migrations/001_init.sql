CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      VARCHAR(32) UNIQUE NOT NULL,
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    bio           TEXT,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS follows (
    follower_id  UUID REFERENCES users(id) ON DELETE CASCADE,
    following_id UUID REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (follower_id, following_id)
);

CREATE TABLE IF NOT EXISTS thoughts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id  UUID REFERENCES users(id) ON DELETE CASCADE,
    subject_id UUID REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (author_id, subject_id)
);

-- audience_id NULL = versione default per "tutti gli altri"
CREATE TABLE IF NOT EXISTS thought_versions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thought_id  UUID REFERENCES thoughts(id) ON DELETE CASCADE,
    audience_id UUID REFERENCES users(id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (thought_id, audience_id)
);

CREATE TABLE IF NOT EXISTS curious_requests (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thought_id   UUID REFERENCES thoughts(id) ON DELETE CASCADE,
    requester_id UUID REFERENCES users(id) ON DELETE CASCADE,
    status       VARCHAR(16) DEFAULT 'pending',
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (thought_id, requester_id)
);

CREATE TABLE IF NOT EXISTS notifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    type       VARCHAR(32) NOT NULL,
    payload    JSONB,
    read       BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
