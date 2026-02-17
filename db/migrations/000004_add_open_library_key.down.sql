DROP INDEX uq_books_open_library_key ON books;
ALTER TABLE books DROP COLUMN open_library_key;

