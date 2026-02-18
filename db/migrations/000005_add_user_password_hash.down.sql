DROP INDEX idx_users_email ON users;
ALTER TABLE users DROP COLUMN password_hash;
