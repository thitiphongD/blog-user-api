-- DropForeignKey
ALTER TABLE "public"."blogs" DROP CONSTRAINT "blogs_user_id_fkey";

-- DropForeignKey
ALTER TABLE "public"."refresh_tokens" DROP CONSTRAINT "refresh_tokens_user_id_fkey";

-- DropForeignKey
ALTER TABLE "public"."comments" DROP CONSTRAINT "comments_blog_id_fkey";

-- DropForeignKey
ALTER TABLE "public"."comments" DROP CONSTRAINT "comments_user_id_fkey";

-- DropTable
DROP TABLE "public"."users";

-- DropTable
DROP TABLE "public"."blogs";

-- DropTable
DROP TABLE "public"."refresh_tokens";

-- DropTable
DROP TABLE "public"."comments";
