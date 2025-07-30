package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
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
	cfg.fileserverHits.Store(0)
}

func (cfg *apiConfig) ServeHttp(res http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits.Add(1)
}

func main() {
	const port = "8080"
	apiCfg := apiConfig{}
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
		type bodyReq struct {
			Body string `json:"body"`
		}

		type cleanedBody struct {
			CleanedBody string `json:"cleaned_body"`
		}
		decoder := json.NewDecoder(req.Body)
		body := bodyReq{}
		err := decoder.Decode(&body)
		if err != nil {
			log.Printf("Error decoding parameters: %s", err)
			res.WriteHeader(500)
			return
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

	server := http.Server{Handler: mux, Addr: ":" + port}
	log.Printf("Serving on port: %s\n", port)
	log.Fatal(server.ListenAndServe())
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
