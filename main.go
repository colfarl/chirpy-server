package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/colfarl/chirpy-server/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body	  string    `json:"body"`
	UserID	  uuid.UUID `json:"user_id"`
}

type apiConfig struct {
	fileServerHits	atomic.Int32
	db				*database.Queries	
	platform		string
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func( w http.ResponseWriter, req *http.Request){
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, req)	
	})
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	html := fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileServerHits.Load())
	w.Write([]byte(html))
}


func (cfg *apiConfig) handlerReset(w http.ResponseWriter, req *http.Request) {

	//reset hits to 0
	cfg.fileServerHits.Store(0)

	//Clean database
	err := cfg.db.DeleteAllUsers(context.Background())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not delete users", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hits reset to 0, database cleaned"))
}

func (cfg * apiConfig) handlerCreateUser(w http.ResponseWriter, req *http.Request) {

	type parameters struct {
		Email	string `json:"email"`
	}
	
	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't decode parameters", err)
		return
	}
	
	if cfg.platform != "dev" {
		respondWithError(w, http.StatusForbidden, "invalid permissions for endpoint", nil)
		return
	}
	
	userParams := database.CreateUserParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Email: params.Email,
	}
	
	user, err := cfg.db.CreateUser(context.Background(), userParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not put user in database", err)
		return
	}

	taggedUser := User{
		ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email: user.Email,
	}
	respondWithJSON(w, http.StatusCreated, taggedUser) 	
}

func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, req *http.Request){

	type parameters struct {
		Body		string `json:"body"`
		UserID		uuid.UUID`json:"user_id"`
	}
	
	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not decode json", err)
		return
	}
	
	if len(params.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "chirp is too long", err)
		return
	}
	
	cleanChirp := cleanWords(params.Body)
	chirpParams := database.CreateChirpParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Body: cleanChirp,
		UserID: params.UserID,
	}
	
	chirp, err := cfg.db.CreateChirp(context.Background(), chirpParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "chirp not uploaded", err)
		return
	}

	respondWithJSON(w, http.StatusCreated, Chirp{
		ID: chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.CreatedAt,
		Body: chirp.Body,
		UserID: chirp.UserID,
	})	
}

func (cfg *apiConfig) handlerGetChirps(w http.ResponseWriter, req *http.Request){

	allChirps := []Chirp{}
	chirpsUnformatted, err := cfg.db.GetAllChirps(context.Background())

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not retrieve chirps", err)
		return
	}

	for _, chirp := range chirpsUnformatted {
		allChirps = append(allChirps, Chirp{
			ID: chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body: chirp.Body,
			UserID: chirp.UserID,
		})
	}
	
	respondWithJSON(w, http.StatusOK, allChirps)
}

func main() {

	const port = "8080"
	const root = "." 	
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")		
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	platform := os.Getenv("PLATFORM")
	dbQueries := database.New(db)

	apiCfg := &apiConfig{
		fileServerHits: atomic.Int32{},
		db: dbQueries,
		platform: platform,
	}

	handler := http.StripPrefix("/app", http.FileServer(http.Dir(root)))
	mux := http.NewServeMux()	
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))
		
	mux.HandleFunc("GET /api/healthz", func (w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	})

	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics) 
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset) 
	mux.HandleFunc("POST /api/users", apiCfg.handlerCreateUser) 
	mux.HandleFunc("POST /api/chirps", apiCfg.handlerCreateChirp) 
	mux.HandleFunc("GET /api/chirps", apiCfg.handlerGetChirps) 

	srv := &http.Server{
		Addr: ":" + port,
		Handler: mux,
	}
	
	fmt.Println("Serving on port", port)
	log.Fatal(srv.ListenAndServe())
}
