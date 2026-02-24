# BookRec – Personalized Book Recommendation Backend

BookRec is a small backend service that learns from a user’s likes and browsing history and recommends books using only SQL and open data. It is designed as a portfolio project to demonstrate practical backend skills using Go, MySQL, and public book APIs.

## Features

- User registration with basic profile (email + handle + password)
- JWT-based authentication (access + refresh tokens)
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
- **Auth:** JWT (`github.com/golang-jwt/jwt/v5`) + refresh token rotation
- **External data:** Open Library public API for seeding books

## Prerequisites

- Go (1.21+)
- Docker Desktop (for MySQL)
- `swag` CLI (for regenerating Swagger docs if you change handlers)

Install `swag` once:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

---

## Getting Started (Local Development)

From the project root:

### 1) Start MySQL with Docker

```bash
docker run --name bookrec-mysql \
  -e MYSQL_ROOT_PASSWORD=root \
  -e MYSQL_DATABASE=bookrec \
  -p 3307:3306 \
  -d mysql:8
```

This gives you a `bookrec` database on `127.0.0.1:3307` with user `root/root`.

### 2) Create environment file

Create `configs/.env`:

```env
DB_USER=root
DB_PASS=root
DB_HOST=127.0.0.1
DB_NAME=bookrec
DB_TLS=false
```

### 3) Apply migrations

If you use the `migrate` CLI:

```bash
migrate -path db/migrations \
  -database "mysql://root:root@tcp(127.0.0.1:3307)/bookrec?multiStatements=true" up
```

Alternatively, run the `.up.sql` files under `db/migrations` manually in your MySQL client.

### 4) Ingest sample books

From the project root:

```bash
go run cmd/jobs/ingest/main.go
```

This job calls the Open Library API, normalises fields, and inserts a small curated catalogue into `books`.

### 5) Run the API server

```bash
go run cmd/server/main.go
```

Server listens on `http://localhost:8080`.

---

## API Overview

### Health and Stats

- `GET /healthz` – simple health check
- `GET /stats` – counts of users, books, interactions

### Books

- `GET /books` – paginated list
  - `page` (query, optional, default `1`)
  - `limit` (query, optional, default `20`, max `100`)
- `GET /books/popular` – most liked books globally
- `GET /books/search` – search + filters + pagination
  - `q` (query, optional)
  - `author` (query, optional)
  - `year_from` (query, optional)
  - `year_to` (query, optional)
  - `sort` (query, optional; e.g. `relevance`, `newest`, `popular`)
  - `page` (query, optional, default `1`)
  - `limit` (query, optional, default `20`, max `100`)

### Users

- `POST /users` – create a new user
  - `email` (x-www-form-urlencoded, required)
  - `handle` (x-www-form-urlencoded, required)
  - `password` (x-www-form-urlencoded, required)
- `GET /users` – list all users
- `GET /users/{id}/history` – last 50 interactions for a user

### Auth

- `POST /login` – login and receive tokens
  - `email` (x-www-form-urlencoded, required)
  - `password` (x-www-form-urlencoded, required)
  - response includes `access_token`, `refresh_token`, and `user`
- `POST /refresh` – rotate tokens using a refresh token
  - `refresh_token` (x-www-form-urlencoded, required)
- `POST /logout` – revoke the provided refresh token
  - `refresh_token` (x-www-form-urlencoded, required)
- `POST /logout-all` – revoke all refresh tokens for the authenticated user
  - requires `Authorization: Bearer <access_token>`

### Interactions (Protected)

- `POST /interactions` – record a user action on a book (**requires auth**)
  - `Authorization: Bearer <access_token>`
  - `user_id` (x-www-form-urlencoded, required)
  - `book_id` (x-www-form-urlencoded, required)
  - `action` (x-www-form-urlencoded, required: `view`, `like`, `rating`)
  - `rating` (x-www-form-urlencoded, optional for the `rating` action)

### Recommendations

- `GET /recommendations/{user_id}` – recommended books for that user, sorted by score

---

## Example Requests

### Create a user (JWT-enabled)

```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "email=testjwt@example.com&handle=testjwt&password=Passw0rd!"
```

### Login

```bash
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "email=testjwt@example.com&password=Passw0rd!"
```

Response example:

```json
{
  "access_token": "<jwt>",
  "refresh_token": "<refresh>",
  "user": { "id": 1, "email": "testjwt@example.com", "role": "user" }
}
```

### List books (page 1, 10 per page)

```bash
curl "http://localhost:8080/books?page=1&limit=10"
```

### Protected: create interaction (like a book)

```bash
curl -X POST http://localhost:8080/interactions \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "user_id=1&book_id=1&action=like"
```

### Fetch recommendations for user 1

```bash
curl http://localhost:8080/recommendations/1
```

### Refresh tokens

```bash
curl -X POST http://localhost:8080/refresh \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "refresh_token=<REFRESH_TOKEN>"
```

### Logout (revoke refresh token)

```bash
curl -X POST http://localhost:8080/logout \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "refresh_token=<REFRESH_TOKEN>"
```

### Logout all sessions (revoke all refresh tokens)

```bash
curl -X POST http://localhost:8080/logout-all \
  -H "Authorization: Bearer <TOKEN>"
```

---

## API Documentation (Swagger)

Swagger UI is served by the Go app.

- UI: `http://localhost:8080/swagger/index.html`
- Raw spec: `http://localhost:8080/swagger/doc.json`

If you change handlers or add endpoints, regenerate docs from project root:

```bash
swag init -g cmd/server/main.go -o docs
```

---

## Notes and Possible Extensions

This project is intentionally focused on fundamentals:

- Clean SQL schema and constraints
- Simple collaborative filtering logic expressed directly in SQL
- Clear separation between API, ingestion job, and database

Ideas for future extensions:

- Move auth refresh token to HttpOnly cookies
- Add more advanced ranking (decay by time, weighting likes vs ratings)
- Add search full-text support
- Move handler logic into `internal/api` and `internal/core` for a stricter layered architecture
- Add Docker Compose to bring up API + MySQL in one command
- Expand the `web/` frontend into a full demo UI
