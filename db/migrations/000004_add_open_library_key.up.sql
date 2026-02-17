-- Add stable external ID (Open Library work key like "/works/OL82563W")
ALTER TABLE books
  ADD COLUMN open_library_key VARCHAR(64) NULL;

-- Unique key makes ingestion idempotent
-- NOTE: MySQL allows multiple NULLs in a UNIQUE index, so existing rows won't break.
CREATE UNIQUE INDEX uq_books_open_library_key ON books(open_library_key);
