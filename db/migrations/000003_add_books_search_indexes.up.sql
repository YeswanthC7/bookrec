-- speed up searches
CREATE INDEX idx_books_title ON books(title);
CREATE INDEX idx_books_author ON books(author);

-- speed up popularity joins/aggregations
CREATE INDEX idx_interactions_book_id ON interactions(book_id);
CREATE INDEX idx_interactions_user_book ON interactions(user_id, book_id);
