package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Load environment variables
	if err := godotenv.Load("configs/.env"); err != nil {
		log.Println("⚠️  No .env file found, using system vars")
	}

	// Build DSN (local MySQL on port 3307)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:3307)/%s?parseTime=true&tls=%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASS"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_TLS"),
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("❌ DB connection error: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ DB unreachable: %v", err)
	}
	log.Println("✅ Connected to MySQL!")

	// Initialize Gin
	r := gin.Default()

	// ---------------- Health Check ----------------
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// ---------------- Users ----------------
	r.POST("/users", func(c *gin.Context) {
		email := c.PostForm("email")
		handle := c.PostForm("handle")

		if email == "" || handle == "" {
			c.JSON(400, gin.H{"error": "email and handle required"})
			return
		}

		_, err := db.Exec("INSERT INTO users (email, handle) VALUES (?, ?)", email, handle)
		if err != nil {
			if strings.Contains(err.Error(), "Duplicate entry") {
				c.JSON(400, gin.H{"error": "Email already exists"})
				return
			}
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"message": "User created"})
	})

	r.GET("/users", func(c *gin.Context) {
		rows, err := db.Query("SELECT id, email, handle, created_at FROM users")
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		users := []map[string]interface{}{}
		for rows.Next() {
			var id int
			var email, handle, createdAt string
			rows.Scan(&id, &email, &handle, &createdAt)
			users = append(users, gin.H{
				"id": id, "email": email, "handle": handle, "created_at": createdAt,
			})
		}
		c.JSON(200, users)
	})

	// ---------------- Books ----------------
	r.GET("/books", func(c *gin.Context) {
		rows, err := db.Query("SELECT id, title, author, published_year FROM books LIMIT 20")
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		books := []map[string]interface{}{}
		for rows.Next() {
			var id, year int
			var title, author string
			rows.Scan(&id, &title, &author, &year)
			books = append(books, gin.H{
				"id": id, "title": title, "author": author, "year": year,
			})
		}
		c.JSON(200, books)
	})

	// ---------------- Interactions ----------------
	r.POST("/interactions", func(c *gin.Context) {
		userID := c.PostForm("user_id")
		bookID := c.PostForm("book_id")
		action := c.PostForm("action") // view | like | rating
		rating := c.PostForm("rating") // optional

		if userID == "" || bookID == "" || action == "" {
			c.JSON(400, gin.H{"error": "user_id, book_id, and action are required"})
			return
		}

		if rating == "" {
			_, err = db.Exec(`
				INSERT INTO interactions (user_id, book_id, action)
				VALUES (?, ?, ?)`,
				userID, bookID, action,
			)
		} else {
			_, err = db.Exec(`
				INSERT INTO interactions (user_id, book_id, action, rating)
				VALUES (?, ?, ?, ?)`,
				userID, bookID, action, rating,
			)
		}
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "Interaction recorded"})
	})

	// ---------------- Recommendations ----------------
	r.GET("/recommendations/:user_id", func(c *gin.Context) {
		userID := c.Param("user_id")

		query := `
		SELECT b.id, b.title, b.author, COUNT(*) AS score
		FROM interactions i
		JOIN interactions j ON i.book_id = j.book_id 
			AND i.user_id != j.user_id
		JOIN books b ON b.id = j.book_id
		WHERE i.user_id = ? AND j.action = 'like'
			AND j.book_id NOT IN (
				SELECT book_id FROM interactions WHERE user_id = ?
			)
		GROUP BY b.id, b.title, b.author
		ORDER BY score DESC
		LIMIT 10;`

		rows, err := db.Query(query, userID, userID)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		recs := []map[string]interface{}{}
		for rows.Next() {
			var id int
			var title, author string
			var score int
			rows.Scan(&id, &title, &author, &score)
			recs = append(recs, gin.H{
				"book_id": id,
				"title":   title,
				"author":  author,
				"score":   score,
			})
		}

		if len(recs) == 0 {
			c.JSON(200, gin.H{"message": "No recommendations yet — like a few books first!"})
			return
		}

		c.JSON(200, recs)
	})

	// Start server
	r.Run(":8080")
}
