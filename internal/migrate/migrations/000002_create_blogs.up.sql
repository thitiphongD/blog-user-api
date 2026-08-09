CREATE TABLE blogs (
    id         UUID        PRIMARY KEY,
    title      TEXT        NOT NULL,
    content    TEXT        NOT NULL,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

-- GORM ต่อ WHERE deleted_at IS NULL ให้ทุก query อยู่แล้ว partial index เลยคุ้มกว่า index เดี่ยว
CREATE INDEX idx_blogs_created_at ON blogs (created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_blogs_user_id    ON blogs (user_id)         WHERE deleted_at IS NULL;
