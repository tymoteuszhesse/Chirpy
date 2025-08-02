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

	"github.com/gofrs/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/tymoteuszhesse/chirpy/internal/database"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	platform       string
	dbQueries      *database.Queries
}

type bodyReq struct {
	Body string `json:"body"`
}

type cleanedBody struct {
	CleanedBody string `json:"cleaned_body"`
}

type userCreate struct {
	Email string `json:"email"`
}
type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
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

	respondWithJSON(res, 200, []byte("OK"))
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
	log.Println(dbQueries)
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

	mux.HandleFunc("POST /api/validate_chirp", func(res http.ResponseWriter, req *http.Request) {

		decoder := json.NewDecoder(req.Body)
		body := bodyReq{}
		err := decoder.Decode(&body)
		if err != nil {
			handleDecodingError(err, res)
		}
		if len(body.Body) > 140 {
			respondWithError(res, 400, "Chirp is too long")
		}

		body.Body = cleanBadWords(body.Body)

		cleanedResp := cleanedBody{
			CleanedBody: body.Body,
		}
		resp, err := json.Marshal(cleanedResp)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			res.WriteHeader(500)
			return
		}
		respondWithJSON(res, 200, resp)
	})

	mux.HandleFunc("POST /api/users", func(res http.ResponseWriter, req *http.Request) {

		decoder := json.NewDecoder(req.Body)
		body := userCreate{}
		err := decoder.Decode(&body)
		if err != nil {
			handleDecodingError(err, res)
		}
		user, err := dbQueries.CreateUser(req.Context(), body.Email)
		if err != nil {
			log.Printf("Error inserting user: %s", err)
			res.WriteHeader(500)
			return
		}
		userResponse := User{
			ID:        uuid.UUID(user.ID),
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Email:     user.Email,
		}
		resp, err := json.Marshal(userResponse)
		if err != nil {
			log.Printf("Error marshalling response %s", err)
			res.WriteHeader(500)
			return
		}
		respondWithJSON(res, 201, resp)
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

func respondWithError(res http.ResponseWriter, code int, msg string) {
	type errResp struct {
		Error string `json:"error"`
	}
	respBody := errResp{
		Error: msg,
	}
	resp, err := json.Marshal(respBody)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		res.WriteHeader(500)
		return
	}
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(code)
	res.Write(resp)
}

func respondWithJSON(res http.ResponseWriter, code int, payload interface{}) {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(code)
	res.Write(payload.([]byte))
}
