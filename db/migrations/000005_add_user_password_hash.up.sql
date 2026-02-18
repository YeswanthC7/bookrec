ALTER TABLE users
  ADD COLUMN password_hash VARCHAR(255) NOT NULL DEFAULT '' AFTER handle;

CREATE INDEX idx_users_email ON users(email);
