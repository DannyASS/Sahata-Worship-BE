ALTER TABLE team_members DROP FOREIGN KEY fk_team_members_user;
ALTER TABLE team_members DROP INDEX uq_team_members_room_user;
ALTER TABLE team_members DROP COLUMN user_id;

