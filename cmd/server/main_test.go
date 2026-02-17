package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// minimal routes to test
	r.GET("/healthz", HealthHandler)
	r.GET("/stats", StatsHandler)
	r.GET("/books", ListBooksHandler)
	r.GET("/books/search", SearchBooksHandler)

	return r
}

func TestHealthHandler(t *testing.T) {
	r := setupRouter()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", body["status"])
	}
}

func TestStatsHandler(t *testing.T) {
	// mock DB
	var mock sqlmock.Sqlmock
	var err error
	db, mock, err = sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer func() { _ = db.Close() }()

	// expectations (order matters because your handler runs 3 QueryRow calls)
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(2))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM books").
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(80))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM interactions").
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(5))

	r := setupRouter()
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}

	if body["users"] != float64(2) || body["books"] != float64(80) || body["interactions"] != float64(5) {
		t.Fatalf("unexpected stats response: %v", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestListBooksHandler(t *testing.T) {
	// mock DB
	var mock sqlmock.Sqlmock
	var err error
	db, mock, err = sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Expect list query with limit+offset args
	mock.ExpectQuery("SELECT id, title, author, published_year\\s+FROM books").
		WithArgs(2, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "author", "published_year"}).
			AddRow(1, "Book A", "Author A", 2001).
			AddRow(2, "Book B", "Author B", 2002))

	r := setupRouter()
	req := httptest.NewRequest(http.MethodGet, "/books?page=1&limit=2", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestSearchBooksHandler_Relevance(t *testing.T) {
	// mock DB
	var mock sqlmock.Sqlmock
	var err error
	db, mock, err = sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Your query contains LIKE args twice + limit + offset
	mock.ExpectQuery("FROM books b").
		WithArgs("%harry%", "%harry%", 5, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "author", "published_year"}).
			AddRow(10, "Harry Something", "Some Author", 2000))

	r := setupRouter()
	req := httptest.NewRequest(http.MethodGet, "/books/search?q=harry&page=1&limit=5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

// Ensure db is treated as *sql.DB even when mocked
var _ *sql.DB = db