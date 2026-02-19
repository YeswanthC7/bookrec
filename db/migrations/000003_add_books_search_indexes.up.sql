-- speed up searches
CREATE INDEX IF NOT EXISTS idx_books_title ON books(title);
CREATE INDEX IF NOT EXISTS idx_books_author ON books(author);

-- speed up popularity joins/aggregations
CREATE INDEX IF NOT EXISTS idx_interactions_book_id ON interactions(book_id);
CREATE INDEX IF NOT EXISTS idx_interactions_user_book ON interactions(user_id, book_id);
