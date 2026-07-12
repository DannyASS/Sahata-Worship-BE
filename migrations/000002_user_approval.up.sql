ALTER TABLE users
  ADD COLUMN account_status ENUM('pending','active','rejected') NOT NULL DEFAULT 'pending' AFTER role;

-- Akun yang sudah ada sebelum fitur approval tetap dapat login.
UPDATE users SET account_status = 'active';

