DROP INDEX IF EXISTS donations_livemode_created_idx;
ALTER TABLE donations DROP COLUMN IF EXISTS livemode;
