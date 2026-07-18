CREATE TABLE room_songs (
  room_id BIGINT UNSIGNED NOT NULL,
  song_id BIGINT UNSIGNED NOT NULL,
  display_order INT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (room_id, song_id),
  CONSTRAINT fk_room_songs_room FOREIGN KEY (room_id) REFERENCES rooms(id) ON DELETE CASCADE,
  CONSTRAINT fk_room_songs_song FOREIGN KEY (song_id) REFERENCES songs(id) ON DELETE CASCADE,
  INDEX idx_room_songs_order (room_id, display_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
