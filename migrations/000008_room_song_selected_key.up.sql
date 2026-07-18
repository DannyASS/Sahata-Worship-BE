ALTER TABLE room_songs
  ADD COLUMN selected_key VARCHAR(16) NULL AFTER song_id;

UPDATE room_songs rs
JOIN songs s ON s.id = rs.song_id
SET rs.selected_key = s.default_key;

ALTER TABLE room_songs
  MODIFY COLUMN selected_key VARCHAR(16) NOT NULL;
