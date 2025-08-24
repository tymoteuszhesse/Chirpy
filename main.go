package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/tymoteuszhesse/chirpy/internal/auth"
	"github.com/tymoteuszhesse/chirpy/internal/database"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	platform       string
	dbQueries      *database.Queries
}

type cleanedBody struct {
	CleanedBody string `json:"cleaned_body"`
}

type userCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type Chirps struct {
	Body   string    `json:"body"`
	UserID uuid.UUID `json:"user_id"`
}

type ChirpResponse struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(res, req)
	})
}
func (cfg *apiConfig) countHandler(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", "text/html")
	res.Write([]byte(fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) resetHandler(res http.ResponseWriter, req *http.Request) {
	if cfg.platform != "dev" {
		respondWithError(res, 403, "Forbidden")
	}
	err := cfg.dbQueries.RemoveUsers(req.Context())
	if err != nil {
		log.Printf("Error removing users: %s", err)
		res.WriteHeader(500)
		return
	}

	respondWithJSON(res, 200, "OK")
	cfg.fileserverHits.Store(0)
}

func (cfg *apiConfig) ServeHttp(res http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits.Add(1)
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, _ := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)
	const port = "8080"
	apiCfg := apiConfig{platform: os.Getenv("PLATFORM"), dbQueries: dbQueries}
	mux := http.NewServeMux()
	handler := http.FileServer(http.Dir("."))
	mux.Handle("/app/", http.StripPrefix("/app", apiCfg.middlewareMetricsInc(handler)))

	mux.HandleFunc("GET /admin/metrics", apiCfg.countHandler)
	mux.HandleFunc("POST /admin/reset", apiCfg.resetHandler)

	mux.HandleFunc("GET /api/healthz", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("Content-Type", "text/plain; charset=utf-8")
		res.WriteHeader(200)
		res.Write([]byte("OK"))
	})

	mux.HandleFunc("POST /api/users", func(res http.ResponseWriter, req *http.Request) {
		decoder := json.NewDecoder(req.Body)
		body := userCredentials{}
		err := decoder.Decode(&body)
		if err != nil {
			handleDecodingError(err, res)
		}
		hashedPassword, err := auth.HashPassword(body.Password)
		if err != nil {
			log.Printf("Error hashing password: %s", err)
			res.WriteHeader(500)
			return
		}
		user, err := dbQueries.CreateUser(req.Context(), database.CreateUserParams{Email: body.Email, HashedPassword: hashedPassword})
		if err != nil {
			log.Printf("Error inserting user: %s", err)
			res.WriteHeader(500)
			return
		}
		userUUID, err := uuid.Parse(user.ID.String())
		if err != nil {
			log.Printf("Error converting user ID to UUID: %s", err)
			res.WriteHeader(500)
			return
		}

		respondWithJSON(res, 201, User{
			ID:        userUUID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Email:     user.Email,
		})
	})

	mux.HandleFunc("POST /api/login", func(res http.ResponseWriter, req *http.Request) {
		decoder := json.NewDecoder(req.Body)
		body := userCredentials{}
		err := decoder.Decode(&body)
		if err != nil {
			handleDecodingError(err, res)
		}
		user, err := dbQueries.GetUserPassword(req.Context(), body.Email)
		if err != nil {
			log.Printf("Error inserting user: %s", err)
			res.WriteHeader(500)
			return
		}
		err = auth.CheckPasswordHash(body.Password, user.HashedPassword)
		if err != nil {
			res.WriteHeader(401)
			res.Write([]byte("Unauthorized"))
			return
		}

		respondWithJSON(res, 200, User{
			ID:        user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Email:     user.Email,
		})
	})

	mux.HandleFunc("POST /api/chirps", func(res http.ResponseWriter, req *http.Request) {

		decoder := json.NewDecoder(req.Body)
		body := Chirps{}
		err := decoder.Decode(&body)
		if err != nil {
			handleDecodingError(err, res)
		}

		if len(body.Body) > 140 {
			respondWithError(res, 400, "Chirp is too long")
		}

		body.Body = cleanBadWords(body.Body)
		chirp, _ := dbQueries.CreateChirp(req.Context(), database.CreateChirpParams{UserID: body.UserID, Body: body.Body})

		chirpID, err := uuid.Parse(chirp.ID.String())
		if err != nil {
			log.Printf("Error converting chirp ID to UUID: %s", err)
			res.WriteHeader(500)
			return
		}
		userID, err := uuid.Parse(chirp.UserID.String())
		if err != nil {
			log.Printf("Error converting user ID to UUID: %s", err)
			res.WriteHeader(500)
			return
		}

		respondWithJSON(res, 201, ChirpResponse{ID: chirpID, CreatedAt: chirp.CreatedAt, UpdatedAt: chirp.UpdatedAt, Body: chirp.Body, UserID: userID})

	})

	mux.HandleFunc("GET /api/chirps", func(res http.ResponseWriter, req *http.Request) {
		chirps, err := dbQueries.GetChirps(req.Context())
		parsedChirps := make([]ChirpResponse, 0, len(chirps))
		if err != nil {
			respondWithError(res, 400, err.Error())
		}
		for _, chirp := range chirps {
			parsedChirps = append(parsedChirps, ChirpResponse{
				ID:        chirp.ID,
				CreatedAt: chirp.CreatedAt,
				UpdatedAt: chirp.UpdatedAt,
				Body:      chirp.Body,
				UserID:    chirp.UserID})
		}

		respondWithJSON(res, 200, parsedChirps)
	})

	mux.HandleFunc("GET /api/chirps/{chirpID}", func(res http.ResponseWriter, req *http.Request) {
		uuidParam := req.PathValue("chirpID")
		uuid, err := uuid.Parse(uuidParam)
		if err != nil {
			respondWithError(res, 400, err.Error())
		}
		chirp, err := dbQueries.GetChirpByID(req.Context(), uuid)
		if err != nil {
			respondWithError(res, 404, "not found")
		}

		respondWithJSON(res, 200, ChirpResponse{ID: chirp.ID, CreatedAt: chirp.CreatedAt, UpdatedAt: chirp.UpdatedAt, Body: chirp.Body, UserID: chirp.UserID})
	})

	server := http.Server{Handler: mux, Addr: ":" + port}
	log.Printf("Serving on port: %s\n", port)
	log.Fatal(server.ListenAndServe())
}

func handleDecodingError(err error, res http.ResponseWriter) {
	log.Printf("Error decoding parameters: %s", err)
	res.WriteHeader(500)
}

func cleanBadWords(text string) string {
	words := strings.Split(text, " ")
	bannedWords := []string{"kerfuffle", "sharbert", "fornax"}
	for _, word := range words {
		for _, bannedWord := range bannedWords {
			if strings.ToLower(word) == bannedWord {
				text = strings.ReplaceAll(text, word, "****")
			}
		}
	}
	return text
}
