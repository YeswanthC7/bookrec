package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

// Book represents one document from the Open Library API
// NOTE: Open Library Search API returns a stable work key in "key" (e.g., "/works/OL82563W")
type Book struct {
	Key      string   `json:"key"` // ✅ added for idempotent upserts
	Title    string   `json:"title"`
	Authors  []string `json:"author_name"`
	Subjects []string `json:"subject"`
	Year     int      `json:"first_publish_year"`
}

// SearchResponse represents the overall JSON structure
type SearchResponse struct {
	Docs []Book `json:"docs"`
}

func main() {
	// Load environment variables
	if err := godotenv.Load("configs/.env"); err != nil {
		log.Println("⚠️  No .env file found; using system vars")
	}

	// Build DSN (local MySQL on port 3307)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:3307)/%s?parseTime=true&tls=%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASS"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_TLS"),
	)

	// Connect to DB
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("❌ Failed to open DB: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Cannot reach DB: %v", err)
	}
	log.Println("✅ Connected to MySQL (local Docker container)")

	// Categories to fetch (optional: add demo-friendly queries like "harry+potter", "j+k+rowling")
	categories := []string{
		"science+fiction",
		"data+science",
		"fantasy",
		"self+help",
	}

	for _, cat := range categories {
		url := fmt.Sprintf("https://openlibrary.org/search.json?q=%s&limit=10", cat)
		log.Printf("📥 Fetching: %s\n", url)

		resp, err := http.Get(url)
		if err != nil {
			log.Printf("⚠️  HTTP request failed for %s: %v", cat, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result SearchResponse
		if err := json.Unmarshal(body, &result); err != nil {
			log.Printf("⚠️  JSON decode failed for %s: %v", cat, err)
			continue
		}

		insertCount := 0
		for _, b := range result.Docs {
			if strings.TrimSpace(b.Title) == "" {
				continue
			}

			// ✅ Key is required for dedupe/upsert via UNIQUE(open_library_key)
			if strings.TrimSpace(b.Key) == "" {
				continue
			}

			author := ""
			if len(b.Authors) > 0 {
				author = b.Authors[0]
			}

			subjectsJSON, _ := json.Marshal(b.Subjects)

			_, err := db.Exec(`
				INSERT INTO books (open_library_key, title, author, subjects, published_year)
				VALUES (?, ?, ?, ?, ?)
				ON DUPLICATE KEY UPDATE
					title = VALUES(title),
					author = VALUES(author),
					subjects = VALUES(subjects),
					published_year = VALUES(published_year)`,
				strings.TrimSpace(b.Key),
				strings.TrimSpace(b.Title),
				author,
				string(subjectsJSON),
				b.Year,
			)
			if err != nil {
				log.Printf("❌ Insert failed for '%s': %v", b.Title, err)
				continue
			}
			insertCount++
		}

		log.Printf("✅ Done category: %s (%d books added/updated)", cat, insertCount)
	}

	log.Println("🎉 Book ingestion complete!")
}
