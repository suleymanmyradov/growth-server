-- Remove unique constraint for article shares
ALTER TABLE article_shares DROP CONSTRAINT IF EXISTS unique_article_share_per_platform;
