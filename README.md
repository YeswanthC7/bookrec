
# BookRec – Personalized Book Recommendation Backend

BookRec is a small backend service that learns from a user’s likes and browsing history and recommends books using only SQL and open data. It is designed as a portfolio project to demonstrate practical backend skills using Go, MySQL, and public book APIs.

## Features

- User registration with basic profile (email + handle)
- Book catalogue populated from a free public API
- Tracking of user interactions  
  - views, likes, optional 1–5 rating
- SQL-only recommendation engine  
  - “people who liked the same books as you also liked…”
- System stats endpoint (users, books, interactions)
- Search and pagination for books
- Interactive API documentation with Swagger UI

## Tech Stack

- **Language:** Go
- **Framework:** Gin
- **Database:** MySQL 8 (Docker container for local dev)
- **Migrations:** SQL files under `db/migrations`
- **Docs:** Swagger / OpenAPI (`swaggo/gin-swagger`)
- **External data:** Open Library public API for seeding books

## High-Level Architecture

- `cmd/server/main.go` – HTTP API entry point (Gin router + handlers)
- `cmd/jobs/ingest/main.go` – one-off job to pull book data from Open Library and insert into `books`
- `db/migrations` – schema for `books`, `users`, `interactions`
- `configs/.env` – database connection configuration
- `internal/*` – planned packages for a more layered design (`api`, `core`, `store`, `external`, `recommend`)

The server exposes a REST API on port `8080`. MySQL runs locally (typically via Docker) on port `3307`, and the Go app connects using environment variables.

## Data Model

**books**

- `id` – primary key  
- `title`  
- `author`  
- `published_year`

**users**

- `id` – primary key  
- `email` – unique  
- `handle`  
- `created_at`

**interactions**

- `id` – primary key  
- `user_id` – foreign key → `users.id`  
- `book_id` – foreign key → `books.id`  
- `action` – `view`, `like` or `rating`  
- `rating` – nullable 1–5  
- `created_at`

Recommendations are computed with a SQL query that looks at:

1. Books the current user liked  
2. Other users who liked the same books  
3. Other books those similar users liked that the current user has not interacted with yet

The query aggregates these candidate books and ranks them by a `score` derived from the number of supporting interactions.

## Prerequisites

- Go (1.21+)
- Docker Desktop (for MySQL)
- `swag` CLI (for regenerating Swagger docs if you change handlers)

Install `swag` once:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

## Getting Started (Local Development)

From the project root:

### 1. Start MySQL with Docker

```bash
docker run --name bookrec-mysql \
  -e MYSQL_ROOT_PASSWORD=root \
  -e MYSQL_DATABASE=bookrec \
  -p 3307:3306 \
  -d mysql:8
```

This gives you a `bookrec` database on `127.0.0.1:3307` with user `root/root`.

### 2. Create environment file

Create `configs/.env`:

```env
DB_USER=root
DB_PASS=root
DB_HOST=127.0.0.1
DB_NAME=bookrec
DB_TLS=false
```

### 3. Apply migrations

If you use the `migrate` CLI:

```bash
migrate -path db/migrations \
  -database "mysql://root:root@tcp(127.0.0.1:3307)/bookrec?multiStatements=true" up
```

Alternatively, run the `.up.sql` files under `db/migrations` manually in your MySQL client.

### 4. Ingest sample books

From the project root:

```bash
go run cmd/jobs/ingest/main.go
```

This job calls the Open Library API, normalises fields, and inserts a small curated catalogue into `books`.

### 5. Run the API server

```bash
go run cmd/server/main.go
```

Server will listen on `http://localhost:8080`.

## API Overview

### Health and Stats

- `GET /healthz` – simple health check
- `GET /stats` – counts of users, books, interactions

### Users

- `POST /users` – create a new user  
  - `email` (form-data, required)  
  - `handle` (form-data, required)  
- `GET /users` – list all users
- `GET /users/{id}/history` – last 50 interactions for a user

### Books

- `GET /books` – paginated list  
  - `page` (query, optional, default `1`)  
  - `limit` (query, optional, default `20`, max `100`)  
- `GET /books/popular` – most liked books globally

### Interactions

- `POST /interactions` – record a user action on a book  
  - `user_id` (form-data, required)  
  - `book_id` (form-data, required)  
  - `action` (form-data, required: `view`, `like`, `rating`)  
  - `rating` (form-data, optional for the `rating` action)

### Recommendations

- `GET /recommendations/{user_id}` – recommended books for that user, sorted by score

## Example Requests

Create a user:

```bash
curl -X POST http://localhost:8080/users \
  -d "email=user1@example.com" \
  -d "handle=user1"
```

List books (first page, 10 per page):

```bash
curl "http://localhost:8080/books?page=1&limit=10"
```

Record that a user liked a book:

```bash
curl -X POST http://localhost:8080/interactions \
  -d "user_id=1" \
  -d "book_id=5" \
  -d "action=like"
```

Fetch recommendations for user `1`:

```bash
curl http://localhost:8080/recommendations/1
```

## API Documentation (Swagger)

Swagger UI is served by the Go app.

- UI: `http://localhost:8080/swagger/index.html`
- Raw spec: `http://localhost:8080/swagger/doc.json`

If you change handlers or add endpoints, regenerate docs from project root:

```bash
swag init -g cmd/server/main.go -o docs
```

## Notes and Possible Extensions

This project is intentionally focused on fundamentals:

- Clean SQL schema and constraints
- Simple collaborative filtering logic expressed directly in SQL
- Clear separation between API, ingestion job, and database

Ideas for future extensions:

- JWT-based authentication and per-user API keys
- More advanced ranking (decay by time, weighting likes vs ratings)
- Search endpoint with full-text support
- Moving handler logic into `internal/api` and `internal/core` for a stricter layered architecture
- Docker Compose file to bring up API + MySQL in one command

## Auth (JWT)

### Create user
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "email=testjwt@example.com&handle=testjwt&password=Passw0rd!"

### Login
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "email=testjwt@example.com&password=Passw0rd!"

# => copy token from response

### Protected: create interaction
curl -X POST http://localhost:8080/interactions \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "user_id=1&book_id=1&action=like"
