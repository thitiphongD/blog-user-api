CREATE TABLE comments (
    id         UUID        PRIMARY KEY,
    content    TEXT        NOT NULL,
    -- ลบ blog แบบถาวรเมื่อไหร่ comment ต้องหายตาม ไม่มีเหตุผลให้ค้างอยู่โดยไม่มีโพสต์
    blog_id    UUID        NOT NULL REFERENCES blogs (id) ON DELETE CASCADE,
    -- RESTRICT เหมือน blogs — คนเขียนคือส่วนหนึ่งของเนื้อหา ลบ user ทิ้งเฉยๆ ไม่ได้
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

-- query หลักคือ "comment ของ blog นี้ เรียงตามเวลา" — index คู่ตรงกับรูปนั้นเลย
CREATE INDEX idx_comments_blog_id ON comments (blog_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_comments_user_id ON comments (user_id)                  WHERE deleted_at IS NULL;
