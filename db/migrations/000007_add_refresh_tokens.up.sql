CREATE TABLE IF NOT EXISTS refresh_tokens (  
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  token_hash VARCHAR(255) NOT NULL,
  expires_at DATETIME NOT NULL,
  revoked_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  UNIQUE KEY uniq_refresh_token_hash (token_hash),
  INDEX idx_refresh_user_id (user_id),
  INDEX idx_refresh_expires_at (expires_at)
);
