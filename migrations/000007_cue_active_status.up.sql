ALTER TABLE cues
  ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE AFTER vibration,
  ADD INDEX idx_cues_active_sort (is_active, sort_order);
