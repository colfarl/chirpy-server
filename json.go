package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func respondWithError(w http.ResponseWriter, code int, msg string, err error) {

	if err != nil {
		log.Println(err)
	}
	if code > 499 {
		log.Printf("Responding with %d error %s", code, msg)
	}

	type errorResponse struct {
		Error		string `json:"error"`
	}
	
	respondWithJSON(w, code, errorResponse{
		Error: msg,
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload any){

	w.Header().Set("Content-Type", "applications/json")
	dat, err := json.Marshal(payload)

	if err != nil {
		log.Printf("error marhalling JSON: %s\n", err)
		w.WriteHeader(500)
		return
	}
	
	w.WriteHeader(code)
	w.Write(dat)
}
