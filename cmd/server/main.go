package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	// Swagger
	_ "github.com/YeswanthC7/bookrec/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// global DB handle for handlers
var db *sql.DB

// JWT config
var jwtSecret []byte
var jwtIssuer string

// Refresh token config
var refreshTokenTTL = 30 * 24 * time.Hour // 30 days

type AuthClaims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         gin.H  `json:"user"`
}

type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type LogoutResponse struct {
	Message string `json:"message"`
}

func generateToken(userID int, email string) (string, error) {
	now := time.Now()
	claims := AuthClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func hashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func newRefreshToken() (plain string, tokenHash string, expiresAt time.Time, err error) {
	// 32 bytes => 256-bit random
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", time.Time{}, err
	}

	plain = base64.RawURLEncoding.EncodeToString(b)
	tokenHash = hashRefreshToken(plain)
	expiresAt = time.Now().Add(refreshTokenTTL)
	return plain, tokenHash, expiresAt, nil
}

func insertRefreshToken(userID int, tokenHash string, expiresAt time.Time) error {
	_, err := db.Exec(`
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES (?, ?, ?)`,
		userID, tokenHash, expiresAt)
	return err
}

func revokeRefreshTokenByID(id int) error {
	_, err := db.Exec(`
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE id = ? AND revoked_at IS NULL`,
		id)
	return err
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid Authorization header"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.ParseWithClaims(tokenStr, &AuthClaims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return jwtSecret, nil
		})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := token.Claims.(*AuthClaims)
		if !ok || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		c.Set("auth_user_id", claims.UserID)
		c.Set("auth_email", claims.Email)
		c.Next()
	}
}

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

	// JWT env
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		log.Fatal("❌ JWT_SECRET is required")
	}
	jwtIssuer = os.Getenv("JWT_ISSUER")
	if jwtIssuer == "" {
		jwtIssuer = "bookrec"
	}

	// Optional refresh TTL override (hours)
	if v := strings.TrimSpace(os.Getenv("REFRESH_TOKEN_TTL_HOURS")); v != "" {
		if hours, err := strconv.Atoi(v); err == nil && hours > 0 {
			refreshTokenTTL = time.Duration(hours) * time.Hour
		}
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
	defer func() { _ = db.Close() }()

	r := gin.Default()

	// Routes
	r.GET("/healthz", HealthHandler)
	r.GET("/stats", StatsHandler)

	r.POST("/users", CreateUserHandler)
	r.POST("/login", LoginHandler)

	// Refresh + logout (refresh rotates refresh token every time)
	r.POST("/refresh", RefreshHandler)
	r.POST("/logout", LogoutHandler)

	r.GET("/users", ListUsersHandler)
	r.GET("/users/:id/history", UserHistoryHandler)

	r.GET("/books", ListBooksHandler)
	r.GET("/books/search", SearchBooksHandler)
	r.GET("/books/popular", PopularBooksHandler)

	// Protected
	r.POST("/interactions", AuthMiddleware(), CreateInteractionHandler)

	r.GET("/recommendations/:user_id", RecommendationsHandler)

	// Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	if err := r.Run(":8080"); err != nil {
		log.Fatalf("❌ server failed: %v", err)
	}
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

	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM books").Scan(&bookCount); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM interactions").Scan(&interactionCount); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

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
// @Param password formData string true "Password"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /users [post]
func CreateUserHandler(c *gin.Context) {
	email := strings.TrimSpace(c.PostForm("email"))
	handle := strings.TrimSpace(c.PostForm("handle"))
	password := c.PostForm("password")

	if email == "" || handle == "" || password == "" {
		c.JSON(400, gin.H{"error": "email, handle, and password required"})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to hash password"})
		return
	}

	_, err = db.Exec("INSERT INTO users (email, handle, password_hash) VALUES (?, ?, ?)", email, handle, string(hashed))
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

// LoginHandler godoc
// @Summary Login and get tokens (access + refresh)
// @Tags Auth
// @Accept mpfd
// @Produce json
// @Param email formData string true "Email"
// @Param password formData string true "Password"
// @Success 200 {object} LoginResponse
// @Failure 401 {object} map[string]interface{}
// @Router /login [post]
func LoginHandler(c *gin.Context) {
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")

	if email == "" || password == "" {
		c.JSON(400, gin.H{"error": "email and password required"})
		return
	}

	var userID int
	var passwordHash string
	if err := db.QueryRow("SELECT id, password_hash FROM users WHERE email = ?", email).Scan(&userID, &passwordHash); err != nil {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}

	accessToken, err := generateToken(userID, email)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate access token"})
		return
	}

	refreshPlain, refreshHash, refreshExp, err := newRefreshToken()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate refresh token"})
		return
	}
	if err := insertRefreshToken(userID, refreshHash, refreshExp); err != nil {
		c.JSON(500, gin.H{"error": "failed to store refresh token"})
		return
	}

	c.JSON(200, LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshPlain,
		User:         gin.H{"id": userID, "email": email},
	})
}

// RefreshHandler godoc
// @Summary Refresh tokens (rotates refresh token every call)
// @Tags Auth
// @Accept mpfd
// @Produce json
// @Param refresh_token formData string true "Refresh token"
// @Success 200 {object} RefreshResponse
// @Failure 401 {object} map[string]interface{}
// @Router /refresh [post]
func RefreshHandler(c *gin.Context) {
	refreshToken := strings.TrimSpace(c.PostForm("refresh_token"))
	if refreshToken == "" {
		c.JSON(400, gin.H{"error": "refresh_token required"})
		return
	}

	tokenHash := hashRefreshToken(refreshToken)

	// Validate refresh token row
	var rowID int
	var userID int
	var expiresAt time.Time
	var revokedAt sql.NullTime
	if err := db.QueryRow(`
		SELECT id, user_id, expires_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = ?
		LIMIT 1`, tokenHash).Scan(&rowID, &userID, &expiresAt, &revokedAt); err != nil {
		c.JSON(401, gin.H{"error": "invalid refresh token"})
		return
	}

	if revokedAt.Valid {
		c.JSON(401, gin.H{"error": "refresh token revoked"})
		return
	}
	if time.Now().After(expiresAt) {
		_ = revokeRefreshTokenByID(rowID) // best-effort cleanup
		c.JSON(401, gin.H{"error": "refresh token expired"})
		return
	}

	// Get email for JWT claims
	var email string
	if err := db.QueryRow(`SELECT email FROM users WHERE id = ?`, userID).Scan(&email); err != nil {
		c.JSON(401, gin.H{"error": "invalid refresh token user"})
		return
	}

	// Rotation: revoke old token, issue new refresh + new access
	if err := revokeRefreshTokenByID(rowID); err != nil {
		c.JSON(500, gin.H{"error": "failed to rotate refresh token"})
		return
	}

	newPlain, newHash, newExp, err := newRefreshToken()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate new refresh token"})
		return
	}
	if err := insertRefreshToken(userID, newHash, newExp); err != nil {
		c.JSON(500, gin.H{"error": "failed to store new refresh token"})
		return
	}

	accessToken, err := generateToken(userID, email)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate access token"})
		return
	}

	c.JSON(200, RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: newPlain,
	})
}

// LogoutHandler godoc
// @Summary Logout (revoke refresh token)
// @Tags Auth
// @Accept mpfd
// @Produce json
// @Param refresh_token formData string true "Refresh token"
// @Success 200 {object} LogoutResponse
// @Failure 401 {object} map[string]interface{}
// @Router /logout [post]
func LogoutHandler(c *gin.Context) {
	refreshToken := strings.TrimSpace(c.PostForm("refresh_token"))
	if refreshToken == "" {
		c.JSON(400, gin.H{"error": "refresh_token required"})
		return
	}

	tokenHash := hashRefreshToken(refreshToken)

	// Revoke best-effort; if token doesn't exist treat as invalid
	res, err := db.Exec(`
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE token_hash = ? AND revoked_at IS NULL`, tokenHash)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to revoke refresh token"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.JSON(401, gin.H{"error": "invalid refresh token"})
		return
	}

	c.JSON(200, LogoutResponse{Message: "Logged out"})
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
	defer func() { _ = rows.Close() }()

	users := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var email, handle, createdAt string
		if err := rows.Scan(&id, &email, &handle, &createdAt); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
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
	defer func() { _ = rows.Close() }()

	books := []map[string]interface{}{}
	for rows.Next() {
		var id, year int
		var title, author string
		if err := rows.Scan(&id, &title, &author, &year); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
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
	defer func() { _ = rows.Close() }()

	popular := []map[string]interface{}{}
	for rows.Next() {
		var id, likes int
		var title, author string
		if err := rows.Scan(&id, &title, &author, &likes); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
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
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
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

	// Enforce token user == form user_id (prevents spoofing)
	authUserIDAny, exists := c.Get("auth_user_id")
	if !exists {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	authUserID, ok := authUserIDAny.(int)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	uid, err := strconv.Atoi(userID)
	if err != nil || uid <= 0 {
		c.JSON(400, gin.H{"error": "invalid user_id"})
		return
	}
	if uid != authUserID {
		c.JSON(403, gin.H{"error": "cannot create interaction for another user"})
		return
	}

	var execErr error
	if rating == "" {
		_, execErr = db.Exec(`
            INSERT INTO interactions (user_id, book_id, action)
            VALUES (?, ?, ?)`,
			userID, bookID, action)
	} else {
		_, execErr = db.Exec(`
            INSERT INTO interactions (user_id, book_id, action, rating)
            VALUES (?, ?, ?, ?)`,
			userID, bookID, action, rating)
	}

	if execErr != nil {
		c.JSON(500, gin.H{"error": execErr.Error()})
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
	defer func() { _ = rows.Close() }()

	history := []map[string]interface{}{}
	for rows.Next() {
		var id, bookID int
		var action string
		var rating sql.NullInt64
		var createdAt, title, author string

		if err := rows.Scan(&id, &bookID, &action, &rating, &createdAt, &title, &author); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

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
	defer func() { _ = rows.Close() }()

	recs := []map[string]interface{}{}
	for rows.Next() {
		var id, score int
		var title, author string
		if err := rows.Scan(&id, &title, &author, &score); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
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
		sb.Reset()
		sb.WriteString(`
			SELECT b.id, b.title, b.author, b.published_year, COUNT(i.id) AS likes
			FROM books b
			LEFT JOIN interactions i
				ON i.book_id = b.id AND i.action = 'like'
			WHERE 1=1
		`)

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
		// NOTE: currently "relevance" falls back to newest-by-id
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
	defer func() { _ = rows.Close() }()

	data := []map[string]interface{}{}

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
