package main

import (
    "database/sql"
    "fmt"
    "log"
    "net/http"
    "os"
    "strconv"
    "strings"

    "github.com/gin-gonic/gin"
    _ "github.com/go-sql-driver/mysql"
    "github.com/joho/godotenv"

    // Swagger
    _ "github.com/YeswanthC7/bookrec/docs"
    swaggerFiles "github.com/swaggo/files"
    ginSwagger "github.com/swaggo/gin-swagger"
)

// global DB handle for handlers
var db *sql.DB

// @title BookRec API
// @version 1.0
// @description Backend for personalized book recommendation system
// @host localhost:8080
// @BasePath /
func main() {
    // Load environment variables
    if err := godotenv.Load("configs/.env"); err != nil {
        log.Println("⚠️ No .env file found, using system vars")
    }

    // Build DSN
    dsn := fmt.Sprintf("%s:%s@tcp(%s:3307)/%s?parseTime=true&tls=%s",
        os.Getenv("DB_USER"),
        os.Getenv("DB_PASS"),
        os.Getenv("DB_HOST"),
        os.Getenv("DB_NAME"),
        os.Getenv("DB_TLS"),
    )

    database, err := sql.Open("mysql", dsn)
    if err != nil {
        log.Fatalf("❌ DB connection error: %v", err)
    }
    if err := database.Ping(); err != nil {
        log.Fatalf("❌ DB unreachable: %v", err)
    }
    log.Println("✅ Connected to MySQL!")
    db = database
    defer db.Close()

    r := gin.Default()

    // Routes
    r.GET("/healthz", HealthHandler)
    r.GET("/stats", StatsHandler)

    r.POST("/users", CreateUserHandler)
    r.GET("/users", ListUsersHandler)
    r.GET("/users/:id/history", UserHistoryHandler)

    r.GET("/books", ListBooksHandler)
    r.GET("/books/search", SearchBooksHandler)
    r.GET("/books/popular", PopularBooksHandler)

    r.POST("/interactions", CreateInteractionHandler)

    r.GET("/recommendations/:user_id", RecommendationsHandler)

    // Swagger UI
    r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

    r.Run(":8080")
}

//
// -------- Handlers with Swagger annotations --------
//

// HealthHandler godoc
// @Summary Health Check
// @Description Returns status of the server
// @Tags System
// @Success 200 {object} map[string]interface{}
// @Router /healthz [get]
func HealthHandler(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// StatsHandler godoc
// @Summary System stats (counts)
// @Tags System
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /stats [get]
func StatsHandler(c *gin.Context) {
    var userCount, bookCount, interactionCount int

    db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
    db.QueryRow("SELECT COUNT(*) FROM books").Scan(&bookCount)
    db.QueryRow("SELECT COUNT(*) FROM interactions").Scan(&interactionCount)

    c.JSON(200, gin.H{
        "users":        userCount,
        "books":        bookCount,
        "interactions": interactionCount,
    })
}

// CreateUserHandler godoc
// @Summary Create a new user
// @Description Registers a new user
// @Tags Users
// @Accept mpfd
// @Produce json
// @Param email formData string true "Email"
// @Param handle formData string true "Handle"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /users [post]
func CreateUserHandler(c *gin.Context) {
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
}

// ListUsersHandler godoc
// @Summary List all users
// @Tags Users
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /users [get]
func ListUsersHandler(c *gin.Context) {
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
            "id":         id,
            "email":      email,
            "handle":     handle,
            "created_at": createdAt,
        })
    }
    c.JSON(200, users)
}

// ListBooksHandler godoc
// @Summary List books (paginated)
// @Tags Books
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Limit"
// @Success 200 {object} map[string]interface{}
// @Router /books [get]
func ListBooksHandler(c *gin.Context) {
    pageStr := c.DefaultQuery("page", "1")
    limitStr := c.DefaultQuery("limit", "20")

    page, _ := strconv.Atoi(pageStr)
    if page < 1 {
        page = 1
    }

    limit, _ := strconv.Atoi(limitStr)
    if limit < 1 || limit > 100 {
        limit = 20
    }

    offset := (page - 1) * limit

    query := `
        SELECT id, title, author, published_year 
        FROM books
        ORDER BY id
        LIMIT ? OFFSET ?;
    `
    rows, err := db.Query(query, limit, offset)
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
            "id":     id,
            "title":  title,
            "author": author,
            "year":   year,
        })
    }

    c.JSON(200, gin.H{
        "page":  page,
        "limit": limit,
        "data":  books,
    })
}

// PopularBooksHandler godoc
// @Summary Most popular books
// @Tags Books
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /books/popular [get]
func PopularBooksHandler(c *gin.Context) {
    query := `
        SELECT b.id, b.title, b.author, COUNT(i.id) AS likes
        FROM interactions i
        JOIN books b ON b.id = i.book_id
        WHERE i.action = 'like'
        GROUP BY b.id, b.title, b.author
        ORDER BY likes DESC
        LIMIT 10;
    `
    rows, err := db.Query(query)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    popular := []map[string]interface{}{}
    for rows.Next() {
        var id, likes int
        var title, author string
        rows.Scan(&id, &title, &author, &likes)
        popular = append(popular, gin.H{
            "id":     id,
            "title":  title,
            "author": author,
            "likes":  likes,
        })
    }

    c.JSON(200, popular)
}

// CreateInteractionHandler godoc
// @Summary Record interaction
// @Tags Interactions
// @Accept mpfd
// @Produce json
// @Param user_id formData int true "User ID"
// @Param book_id formData int true "Book ID"
// @Param action formData string true "Action: like | view | rating"
// @Param rating formData int false "Rating"
// @Success 200 {object} map[string]interface{}
// @Router /interactions [post]
func CreateInteractionHandler(c *gin.Context) {
    userID := c.PostForm("user_id")
    bookID := c.PostForm("book_id")
    action := c.PostForm("action")
    rating := c.PostForm("rating")

    if userID == "" || bookID == "" || action == "" {
        c.JSON(400, gin.H{"error": "user_id, book_id, and action are required"})
        return
    }

    var err error
    if rating == "" {
        _, err = db.Exec(`
            INSERT INTO interactions (user_id, book_id, action)
            VALUES (?, ?, ?)`,
            userID, bookID, action)
    } else {
        _, err = db.Exec(`
            INSERT INTO interactions (user_id, book_id, action, rating)
            VALUES (?, ?, ?, ?)`,
            userID, bookID, action, rating)
    }

    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, gin.H{"message": "Interaction recorded"})
}

// UserHistoryHandler godoc
// @Summary Get user interaction history
// @Tags Users
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {array} map[string]interface{}
// @Router /users/{id}/history [get]
func UserHistoryHandler(c *gin.Context) {
    userID := c.Param("id")

    query := `
        SELECT i.id, i.book_id, i.action, i.rating, i.created_at,
               b.title, b.author
        FROM interactions i
        JOIN books b ON b.id = i.book_id
        WHERE i.user_id = ?
        ORDER BY i.created_at DESC
        LIMIT 50;
    `
    rows, err := db.Query(query, userID)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    history := []map[string]interface{}{}
    for rows.Next() {
        var id, bookID int
        var action string
        var rating sql.NullInt64
        var createdAt, title, author string

        rows.Scan(&id, &bookID, &action, &rating, &createdAt, &title, &author)

        var ratingValue interface{}
        if rating.Valid {
            ratingValue = rating.Int64
        } else {
            ratingValue = nil
        }

        history = append(history, gin.H{
            "id":         id,
            "book_id":    bookID,
            "title":      title,
            "author":     author,
            "action":     action,
            "rating":     ratingValue,
            "created_at": createdAt,
        })
    }

    c.JSON(200, history)
}

// RecommendationsHandler godoc
// @Summary Get recommended books for a user
// @Tags Recommendations
// @Produce json
// @Param user_id path int true "User ID"
// @Success 200 {array} map[string]interface{}
// @Router /recommendations/{user_id} [get]
func RecommendationsHandler(c *gin.Context) {
    userID := c.Param("user_id")

    query := `
        SELECT 
            b.id,
            b.title,
            b.author,
            COUNT(*) AS score
        FROM interactions i
        JOIN interactions j 
            ON i.user_id = ?
            AND j.user_id != i.user_id
            AND i.book_id = j.book_id
        JOIN interactions k
            ON k.user_id = j.user_id
        JOIN books b 
            ON b.id = k.book_id
        WHERE i.action = 'like'
        AND j.action = 'like'
        AND k.action = 'like'
        AND k.book_id NOT IN (
            SELECT book_id FROM interactions WHERE user_id = ?
        )
        GROUP BY b.id, b.title, b.author
        ORDER BY score DESC
        LIMIT 10;
    `
    rows, err := db.Query(query, userID, userID)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    recs := []map[string]interface{}{}
    for rows.Next() {
        var id, score int
        var title, author string
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
}
// SearchBooksHandler godoc
// @Summary Search books (filters + pagination)
// @Tags Books
// @Produce json
// @Param q query string false "Keyword in title or author"
// @Param author query string false "Author filter (partial match)"
// @Param year_from query int false "Published year from"
// @Param year_to query int false "Published year to"
// @Param sort query string false "Sort: newest | popular | relevance (default relevance)"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Limit (max 100)" default(20)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /books/search [get]
func SearchBooksHandler(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	author := strings.TrimSpace(c.Query("author"))
	sort := strings.TrimSpace(c.DefaultQuery("sort", "relevance"))

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	yearFromStr := strings.TrimSpace(c.Query("year_from"))
	yearToStr := strings.TrimSpace(c.Query("year_to"))
	yearFrom, _ := strconv.Atoi(yearFromStr)
	yearTo, _ := strconv.Atoi(yearToStr)

	// Base query
	sb := strings.Builder{}
	sb.WriteString(`
		SELECT b.id, b.title, b.author, b.published_year
		FROM books b
		WHERE 1=1
	`)

	args := []interface{}{}

	// Filters
	if q != "" {
		sb.WriteString(" AND (b.title LIKE ? OR b.author LIKE ?)")
		args = append(args, "%"+q+"%", "%"+q+"%")
	}
	if author != "" {
		sb.WriteString(" AND b.author LIKE ?")
		args = append(args, "%"+author+"%")
	}
	if yearFromStr != "" && yearFrom > 0 {
		sb.WriteString(" AND b.published_year >= ?")
		args = append(args, yearFrom)
	}
	if yearToStr != "" && yearTo > 0 {
		sb.WriteString(" AND b.published_year <= ?")
		args = append(args, yearTo)
	}

	// Sorting
	switch sort {
	case "newest":
		sb.WriteString(" ORDER BY b.published_year DESC, b.id DESC")
	case "popular":
		// Popular = number of likes
		// Use LEFT JOIN so books with 0 likes still appear
		// NOTE: We need to rewrite select when joining aggregation
		sb.Reset()
		sb.WriteString(`
			SELECT b.id, b.title, b.author, b.published_year, COUNT(i.id) AS likes
			FROM books b
			LEFT JOIN interactions i
				ON i.book_id = b.id AND i.action = 'like'
			WHERE 1=1
		`)

		// re-apply same filters to this query
		args = []interface{}{}
		if q != "" {
			sb.WriteString(" AND (b.title LIKE ? OR b.author LIKE ?)")
			args = append(args, "%"+q+"%", "%"+q+"%")
		}
		if author != "" {
			sb.WriteString(" AND b.author LIKE ?")
			args = append(args, "%"+author+"%")
		}
		if yearFromStr != "" && yearFrom > 0 {
			sb.WriteString(" AND b.published_year >= ?")
			args = append(args, yearFrom)
		}
		if yearToStr != "" && yearTo > 0 {
			sb.WriteString(" AND b.published_year <= ?")
			args = append(args, yearTo)
		}

		sb.WriteString(" GROUP BY b.id, b.title, b.author, b.published_year")
		sb.WriteString(" ORDER BY likes DESC, b.id DESC")
	default:
		// "relevance" (simple): recent id tie-breaker
		// If q is provided, you can later add scoring logic; for now keep deterministic
		sb.WriteString(" ORDER BY b.id DESC")
	}

	// Pagination
	sb.WriteString(" LIMIT ? OFFSET ?")
	args = append(args, limit, offset)

	rows, err := db.Query(sb.String(), args...)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	data := []map[string]interface{}{}

	// Scan depends on sort=popular (extra likes column)
	if sort == "popular" {
		for rows.Next() {
			var id, year, likes int
			var title, author string
			if err := rows.Scan(&id, &title, &author, &year, &likes); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			data = append(data, gin.H{
				"id":     id,
				"title":  title,
				"author": author,
				"year":   year,
				"likes":  likes,
			})
		}
	} else {
		for rows.Next() {
			var id, year int
			var title, author string
			if err := rows.Scan(&id, &title, &author, &year); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			data = append(data, gin.H{
				"id":     id,
				"title":  title,
				"author": author,
				"year":   year,
			})
		}
	}

	c.JSON(200, gin.H{
		"page":  page,
		"limit": limit,
		"sort":  sort,
		"data":  data,
	})
}
