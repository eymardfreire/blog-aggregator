package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/eymardfreire/blog-aggregator/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	DB *database.Queries
}

type authHandler func(http.ResponseWriter, *http.Request, database.User)

// middlewareAuth authenticates a request, gets the user, and calls the next authed handler
func (cfg *apiConfig) middlewareAuth(handler authHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("Authorization")
		if apiKey == "" {
			respondWithError(w, http.StatusUnauthorized, "Missing API key")
			return
		}
		apiKey = strings.TrimPrefix(apiKey, "ApiKey ")

		user, err := cfg.DB.GetUserByAPIKey(r.Context(), apiKey)
		if err != nil {
			if err == sql.ErrNoRows {
				respondWithError(w, http.StatusUnauthorized, "Invalid API key")
				return
			}
			respondWithError(w, http.StatusInternalServerError, "Server error")
			return
		}

		handler(w, r, user)
	}
}

func main() {
	fmt.Println("Starting application...")

	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Get the port from the environment
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatalf("PORT environment variable is not set")
	}

	// Get the database URL from the environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatalf("DATABASE_URL environment variable is not set")
	}

	fmt.Println("Connecting to the database...")
	// Connect to the database
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}
	defer db.Close()

	// Test the database connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Error pinging the database: %v", err)
	}

	fmt.Println("Successfully connected to the database")

	// Create a new ServeMux
	mux := http.NewServeMux()

	// Create a new database.Queries instance
	queries := database.New(db)

	// Create an apiConfig instance
	apiCfg := apiConfig{
		DB: queries,
	}

	// Define a simple handler function
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, Blog Aggregator!"))
	})

	// Add a readiness handler
	mux.HandleFunc("/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Add an error handler
	mux.HandleFunc("/v1/err", func(w http.ResponseWriter, r *http.Request) {
		respondWithError(w, http.StatusInternalServerError, "Internal Server Error")
	})

	// Add a handler to create a user
	mux.HandleFunc("/v1/users", apiCfg.handleCreateUser)

	// Add a handler to get user info by API key
	mux.HandleFunc("/v1/user", apiCfg.handleGetUserByAPIKey)

	// Add a handler to create a feed
	mux.HandleFunc("/v1/feeds", apiCfg.middlewareAuth(apiCfg.handleCreateFeed))

	// Add a handler to get all feeds
	mux.HandleFunc("/v1/feeds/all", apiCfg.handleGetAllFeeds)

	// Add a handler to create a feed follow
	mux.HandleFunc("/v1/feed_follows", apiCfg.middlewareAuth(apiCfg.handleCreateFeedFollow))

	// Add a handler to delete a feed follow
	mux.HandleFunc("/v1/feed_follows/", apiCfg.middlewareAuth(apiCfg.handleDeleteFeedFollow))

	// Add a handler to get all feed follows for a user
	mux.HandleFunc("/v1/feed_follows/all", apiCfg.middlewareAuth(apiCfg.handleGetAllFeedFollows))

	// Add a handler to get posts by user
	mux.HandleFunc("/v1/posts", apiCfg.middlewareAuth(apiCfg.handleGetPostsByUser))

	// Start the scraper worker
	go startScraperWorker(apiCfg)

	// Start the server
	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

// handleCreateUser handles the creation of a new user
func (apiCfg *apiConfig) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	type request struct {
		Name string `json:"name"`
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	now := time.Now()
	id := uuid.New()

	user, err := apiCfg.DB.CreateUser(r.Context(), database.CreateUserParams{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
		Name:      req.Name,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	respondWithJSON(w, http.StatusCreated, user)
}

// handleGetUserByAPIKey handles retrieving user info by API key
func (apiCfg *apiConfig) handleGetUserByAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	apiKey := r.Header.Get("Authorization")
	if apiKey == "" {
		respondWithError(w, http.StatusUnauthorized, "Missing API key")
		return
	}

	// Ensure the API key is correctly trimmed
	apiKey = strings.TrimPrefix(apiKey, "ApiKey ")
	fmt.Println("Received API Key:", apiKey) // Debug line

	user, err := apiCfg.DB.GetUserByAPIKey(r.Context(), apiKey)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("User not found for API Key:", apiKey) // Debug line
			respondWithError(w, http.StatusNotFound, "User not found")
			return
		}
		fmt.Println("Error retrieving user for API Key:", apiKey, "Error:", err) // Debug line
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve user")
		return
	}

	fmt.Println("User found:", user) // Debug line
	respondWithJSON(w, http.StatusOK, user)
}

// handleCreateFeed handles the creation of a new feed
func (apiCfg *apiConfig) handleCreateFeed(w http.ResponseWriter, r *http.Request, user database.User) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	type request struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	now := time.Now()
	id := uuid.New()

	feed, err := apiCfg.DB.CreateFeed(r.Context(), database.CreateFeedParams{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
		Name:      req.Name,
		Url:       req.URL,
		UserID:    uuid.NullUUID{UUID: user.ID, Valid: true},
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create feed")
		return
	}

	feedFollowID := uuid.New()
	feedFollow, err := apiCfg.DB.CreateFeedFollow(r.Context(), database.CreateFeedFollowParams{
		ID:        feedFollowID,
		CreatedAt: now,
		UpdatedAt: now,
		FeedID:    uuid.NullUUID{UUID: feed.ID, Valid: true},
		UserID:    uuid.NullUUID{UUID: user.ID, Valid: true},
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create feed follow")
		return
	}

	response := map[string]interface{}{
		"feed":        feed,
		"feed_follow": feedFollow,
	}

	respondWithJSON(w, http.StatusCreated, response)
}

// handleGetAllFeeds handles retrieving all feeds
func (apiCfg *apiConfig) handleGetAllFeeds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	feeds, err := apiCfg.DB.GetAllFeeds(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve feeds")
		return
	}

	respondWithJSON(w, http.StatusOK, feeds)
}

// handleCreateFeedFollow handles creating a feed follow
func (apiCfg *apiConfig) handleCreateFeedFollow(w http.ResponseWriter, r *http.Request, user database.User) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	type request struct {
		FeedID uuid.UUID `json:"feed_id"`
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	now := time.Now()
	id := uuid.New()

	feedFollow, err := apiCfg.DB.CreateFeedFollow(r.Context(), database.CreateFeedFollowParams{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
		FeedID:    uuid.NullUUID{UUID: req.FeedID, Valid: true},
		UserID:    uuid.NullUUID{UUID: user.ID, Valid: true},
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create feed follow")
		return
	}

	respondWithJSON(w, http.StatusCreated, feedFollow)
}

// handleDeleteFeedFollow handles deleting a feed follow
func (apiCfg *apiConfig) handleDeleteFeedFollow(w http.ResponseWriter, r *http.Request, user database.User) {
	if r.Method != http.MethodDelete {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	feedFollowID := strings.TrimPrefix(r.URL.Path, "/v1/feed_follows/")
	id, err := uuid.Parse(feedFollowID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid feed follow ID")
		return
	}

	err = apiCfg.DB.DeleteFeedFollow(r.Context(), id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete feed follow")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetAllFeedFollows handles retrieving all feed follows for a user
func (apiCfg *apiConfig) handleGetAllFeedFollows(w http.ResponseWriter, r *http.Request, user database.User) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	feedFollows, err := apiCfg.DB.GetFeedFollowsByUserID(r.Context(), uuid.NullUUID{UUID: user.ID, Valid: true})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve feed follows")
		return
	}

	respondWithJSON(w, http.StatusOK, feedFollows)
}

// handleGetPostsByUser handles retrieving posts by user
func (apiCfg *apiConfig) handleGetPostsByUser(w http.ResponseWriter, r *http.Request, user database.User) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	limitParam := r.URL.Query().Get("limit")
	var limit int
	var err error
	if limitParam != "" {
		limit, err = strconv.Atoi(limitParam)
		if err != nil || limit <= 0 {
			respondWithError(w, http.StatusBadRequest, "Invalid limit parameter")
			return
		}
	} else {
		limit = 10
	}

	posts, err := apiCfg.DB.GetPostsByUserID(r.Context(), database.GetPostsByUserIDParams{
		UserID: uuid.NullUUID{UUID: user.ID, Valid: true},
		Limit:  int32(limit),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve posts")
		return
	}

	respondWithJSON(w, http.StatusOK, posts)
}

// startScraperWorker starts a worker to fetch feeds continuously
func startScraperWorker(cfg apiConfig) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fetchFeeds(cfg)
		}
	}
}

// fetchFeeds fetches the next n feeds that need to be fetched
func fetchFeeds(cfg apiConfig) {
	const n = 10
	feeds, err := cfg.DB.GetNextFeedsToFetch(context.Background(), n)
	if err != nil {
		log.Printf("Failed to get next feeds to fetch: %v", err)
		return
	}

	for _, feed := range feeds {
		processFeed(cfg, feed)
	}
}

// processFeed processes a feed
func processFeed(cfg apiConfig, feed database.Feed) {
	log.Printf("Fetching feed: %s", feed.Name)
	// Fetch and process the feed data here

	// Mark the feed as fetched
	err := cfg.DB.MarkFeedFetched(context.Background(), feed.ID)
	if err != nil {
		log.Printf("Failed to mark feed as fetched: %v", err)
	}
}

// respondWithJSON writes a JSON response
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// respondWithError writes an error response in JSON format
func respondWithError(w http.ResponseWriter, code int, msg string) {
	respondWithJSON(w, code, map[string]string{"error": msg})
}
