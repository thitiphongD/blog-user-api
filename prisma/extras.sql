-- Prisma เขียน partial index (CREATE INDEX ... WHERE ...) ไม่ได้ ไฟล์นี้เลยเขียนมือ
-- แล้วตัว gen ต่อท้าย migration ทุกใบให้ — IF NOT EXISTS เพราะใบที่ไม่ได้แตะ index ต้องไม่พัง
-- (แก้ definition ของ index ที่มีอยู่แล้วต้องเขียน DROP INDEX ในไฟล์ migration เอง)

-- GORM ต่อ WHERE deleted_at IS NULL ให้ทุก query อยู่แล้ว partial index เลยคุ้มกว่า index เดี่ยว
CREATE INDEX IF NOT EXISTS idx_blogs_created_at ON blogs (created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_blogs_user_id    ON blogs (user_id)         WHERE deleted_at IS NULL;

-- query หลักคือ "comment ของ blog นี้ เรียงตามเวลา" — index คู่ตรงกับรูปนั้นเลย
CREATE INDEX IF NOT EXISTS idx_comments_blog_id ON comments (blog_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_comments_user_id ON comments (user_id)                  WHERE deleted_at IS NULL;
