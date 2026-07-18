ALTER TABLE cues
  DROP INDEX idx_cues_active_sort,
  DROP COLUMN is_active;
