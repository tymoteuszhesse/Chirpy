package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func respondWithError(res http.ResponseWriter, code int, msg string) {
	type errResp struct {
		Error string `json:"error"`
	}

	respondWithJSON(res, code, errResp{
		Error: msg,
	})
}

func respondWithJSON(res http.ResponseWriter, code int, payload interface{}) {
	res.Header().Set("Content-Type", "application/json")
	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		res.WriteHeader(500)
		return
	}
	res.WriteHeader(code)
	res.Write(response)
}
