CREATE TABLE songs (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  title VARCHAR(200) NOT NULL,
  artist VARCHAR(160) NOT NULL,
  default_key VARCHAR(16) NOT NULL,
  bpm SMALLINT UNSIGNED NOT NULL,
  created_by BIGINT UNSIGNED NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_songs_created_by FOREIGN KEY (created_by) REFERENCES users(id),
  INDEX idx_songs_title_artist (title, artist)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE song_sections (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  song_id BIGINT UNSIGNED NOT NULL,
  section_label VARCHAR(100) NOT NULL,
  lyrics TEXT NOT NULL,
  display_order INT UNSIGNED NOT NULL DEFAULT 0,
  CONSTRAINT fk_song_sections_song FOREIGN KEY (song_id) REFERENCES songs(id) ON DELETE CASCADE,
  INDEX idx_song_sections_song_order (song_id, display_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE rooms
  ADD COLUMN current_song_id BIGINT UNSIGNED NULL AFTER status,
  ADD COLUMN current_song_section_id BIGINT UNSIGNED NULL AFTER current_song_id,
  ADD CONSTRAINT fk_rooms_current_song FOREIGN KEY (current_song_id) REFERENCES songs(id) ON DELETE SET NULL,
  ADD CONSTRAINT fk_rooms_current_song_section FOREIGN KEY (current_song_section_id) REFERENCES song_sections(id) ON DELETE SET NULL,
  ADD INDEX idx_rooms_current_song (current_song_id),
  ADD INDEX idx_rooms_current_song_section (current_song_section_id);

ALTER TABLE activity_logs
  ADD COLUMN song_id BIGINT UNSIGNED NULL AFTER received,
  ADD COLUMN song_section_id BIGINT UNSIGNED NULL AFTER song_id,
  ADD CONSTRAINT fk_activity_song FOREIGN KEY (song_id) REFERENCES songs(id) ON DELETE SET NULL,
  ADD CONSTRAINT fk_activity_song_section FOREIGN KEY (song_section_id) REFERENCES song_sections(id) ON DELETE SET NULL,
  ADD INDEX idx_activity_song_section (song_section_id);
