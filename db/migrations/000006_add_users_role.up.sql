ALTER TABLE users
  ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'user' AFTER password_hash;

CREATE INDEX idx_users_role ON users(role);
