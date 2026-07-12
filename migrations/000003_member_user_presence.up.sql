ALTER TABLE team_members
  ADD COLUMN user_id BIGINT UNSIGNED NULL AFTER room_id,
  ADD CONSTRAINT fk_team_members_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  ADD UNIQUE KEY uq_team_members_room_user (room_id, user_id);

