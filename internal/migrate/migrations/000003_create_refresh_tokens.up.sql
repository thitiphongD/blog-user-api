CREATE TABLE refresh_tokens (
    id         UUID        PRIMARY KEY,
    -- CASCADE ต่างจาก blogs ที่เป็น RESTRICT ตั้งใจ — ลบ user แล้ว session ต้องตายตาม
    -- ไม่ใช่ค้างเป็น token ที่ยังใช้ได้แต่ไม่มีเจ้าของ
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- เก็บ hash ไม่เก็บ token ดิบ — DB หลุดแล้วต้องเอาไป login ต่อไม่ได้
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

-- ใช้ตอนจับ reuse แล้วต้องเพิกถอนทั้งหมดของ user คนนั้น
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);
