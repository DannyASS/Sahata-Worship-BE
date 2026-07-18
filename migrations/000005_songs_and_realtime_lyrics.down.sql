ALTER TABLE activity_logs DROP FOREIGN KEY fk_activity_song_section, DROP FOREIGN KEY fk_activity_song, DROP INDEX idx_activity_song_section, DROP COLUMN song_section_id, DROP COLUMN song_id;
ALTER TABLE rooms DROP FOREIGN KEY fk_rooms_current_song_section, DROP FOREIGN KEY fk_rooms_current_song, DROP INDEX idx_rooms_current_song_section, DROP INDEX idx_rooms_current_song, DROP COLUMN current_song_section_id, DROP COLUMN current_song_id;
DROP TABLE IF EXISTS song_sections;
DROP TABLE IF EXISTS songs;
