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
	"sort"
	"sync/atomic"
	"time"

	"github.com/colfarl/chirpy-server/internal/auth"
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
	ChirpyRed bool		`json:"is_chirpy_red"`
}

type LoggedInUser struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	ChirpyRed bool		`json:"is_chirpy_red"`
	Token	  string	`json:"token"`
	RefreshToken string `json:"refresh_token"`
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
	secret			string
	polka_key		string
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
		Email		string `json:"email"`
		Password	string `json:"password"`
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

	hash, err := auth.HashPassword(params.Password) 
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to encrypt password", err)
		return
	}

	userParams := database.CreateUserWithPassWordParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Email: params.Email,
		HashedPassword: hash,
	}
	
	user, err := cfg.db.CreateUserWithPassWord(context.Background(), userParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not put user in database", err)
		return
	}

	taggedUser := User{
		ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email: user.Email,
		ChirpyRed: user.IsChirpyRed,
	}

	respondWithJSON(w, http.StatusCreated, taggedUser) 	
}

func (cfg *apiConfig) handlerUserLogin(w http.ResponseWriter, req *http.Request){
	
	type parameters struct {
		Password	string `json:"password"`
		Email		string `json:"email"`
	}
	
	decoder := json.NewDecoder(req.Body)
	params :=  parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not read request", err)
		return
	}
	
	user, err := cfg.db.GetUserByEmail(context.Background(), params.Email)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not find associated user", err)
		return
	}

	err = auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "incorrect email or password", err)
		return
	}
		
	
	token, err := auth.MakeJWT(user.ID, cfg.secret, time.Duration(3600) * time.Second)
	if err != nil {
		log.Println("Error: ", err)
		respondWithError(w, http.StatusInternalServerError, "could not make JWT", err)
	}
	
	refreshToken, _ := auth.MakeRefreshToken()
	refreshParams := database.CreateRefreshTokenParams{
		Token: refreshToken,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID: user.ID,
		ExpiresAt: time.Now().Add(time.Duration(60) * time.Duration(24) * time.Hour),
		RevokedAt: sql.NullTime{
			Time: time.Time{},	
			Valid: false,
		},
	}

	_, err = cfg.db.CreateRefreshToken(context.Background(), refreshParams)
	if err != nil {
		log.Println("error inserting refresh token", refreshParams)
		respondWithError(w, http.StatusInternalServerError, "error creating refresh token", err)
		return
	}

	taggedLoggedInUser := LoggedInUser{
		ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email: user.Email,
		ChirpyRed: user.IsChirpyRed,
		Token: token,
		RefreshToken: refreshToken,
	}
	
	log.Println("user: ", user.ID, "  logged in with token  ", token)
	respondWithJSON(w, http.StatusOK, taggedLoggedInUser)
}

func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, req *http.Request){

	type parameters struct {
		Body		string    `json:"body"`
		UserID		uuid.UUID `json:"user_id"`
	}
	
	log.Println("Request", req.Body)
	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not decode json", err)
		return
	}
	
	token, err := auth.GetBearerToken(req.Header)
	log.Println("request to log in with token ", token, " from user ", params.UserID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "no authorization provided", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		log.Println("Error: ", err, "  Gathered_ID  ", userID, "  Submitted_ID  ", params.UserID)
		respondWithError(w, http.StatusUnauthorized, "invalid token provided", err)
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
		UserID: userID,
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
	
	var chirpsUnformatted []database.Chirp;
	var err error

	authorID := req.URL.Query().Get("author_id")
	authorUUID, _ := uuid.Parse(authorID)
	
	if authorID != "" {
		chirpsUnformatted, err = cfg.db.GetAllChirpsByAuthor(context.Background(), authorUUID)
	} else {
		chirpsUnformatted, err = cfg.db.GetAllChirps(context.Background())
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not retrieve chirps", err)
		return
	}

	sortOrder := req.URL.Query().Get("sort")
	log.Println("Attempting to sort with key ", sortOrder, "len", len(chirpsUnformatted))
	if sortOrder == "asc" {	
		sort.Slice(chirpsUnformatted, func (l, r int) bool {
			return chirpsUnformatted[l].CreatedAt.Before(chirpsUnformatted[r].CreatedAt)
		})
		log.Println("After ", sortOrder, "len", len(chirpsUnformatted))
	}
	if sortOrder == "desc" {
		sort.Slice(chirpsUnformatted, func (l, r int) bool {
			return chirpsUnformatted[l].CreatedAt.After(chirpsUnformatted[r].CreatedAt)
		})
		log.Println("After ", sortOrder, "len", len(chirpsUnformatted))
	}
	
	allChirps := []Chirp{}
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

func (cfg *apiConfig) handlerGetChirp(w http.ResponseWriter, req *http.Request){
	
	chirpIDString := req.PathValue("chirpID")
	chirpID, err := uuid.Parse(chirpIDString)	
	if chirpIDString == "" || err != nil{
		respondWithError(w, http.StatusBadRequest, "could not extract chirp id from url", err)
		return
	}

	unformattedChirp, err := cfg.db.GetOneChirp(context.Background(), chirpID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "chirp does not exist", err)
		return
	}

	respondWithJSON(w, http.StatusOK, Chirp{
		ID: unformattedChirp.ID,
		CreatedAt: unformattedChirp.CreatedAt,
		UpdatedAt: unformattedChirp.UpdatedAt,
		Body: unformattedChirp.Body,
		UserID: unformattedChirp.UserID,
	})
}

func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, req *http.Request){

	token, err := auth.GetBearerToken(req.Header)	
	if err != nil {
		log.Println("could not parse refresh token ", err)
		respondWithError(w, http.StatusBadRequest, "invalid authorization", err)
	}
	
	refreshToken, err := cfg.db.GetRefreshTokenByToken(context.Background(), token)
	if err != nil {
		log.Println("could not find refresh token in database ", err)
		respondWithError(w, http.StatusBadRequest, "invalid authorization", err)
		return
	}
	
	if refreshToken.ExpiresAt.Before(time.Now()) || refreshToken.RevokedAt.Valid  {
		log.Println("user attempted login with invlalidated token")
		respondWithError(w, http.StatusUnauthorized, "session expired", err)
		return
	}

	accessToken, err := auth.MakeJWT(refreshToken.UserID, cfg.secret, time.Duration(3600) * time.Second)
	if err != nil {
		log.Println("Error: ", err)
		respondWithError(w, http.StatusInternalServerError, "could not make JWT", err)
	}

	response :=	struct{
		Token	string `json:"token"`
	} {
		Token: accessToken,
	}

	respondWithJSON(w, http.StatusOK, response)
}

func (cfg *apiConfig) handlerRevoke(w http.ResponseWriter, req *http.Request){

	token, err := auth.GetBearerToken(req.Header)	
	if err != nil {
		log.Println("could not parse refresh token ", err)
		respondWithError(w, http.StatusBadRequest, "invalid authorization", err)
	}
	
	updateParams := database.RevokeTokenParams{
		Token: token,
		UpdatedAt: time.Now(),
	}

	err = cfg.db.RevokeToken(context.Background(), updateParams)
	if err != nil {
		log.Println("tried to revoke token and failed ", err)
		respondWithError(w, http.StatusInternalServerError, "unable to revoke token", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)	
}

func (cfg * apiConfig) handlerUpdateUserInfo(w http.ResponseWriter, req *http.Request){

	token, err := auth.GetBearerToken(req.Header)	
	if err != nil {
		log.Println("could not parse authorization token ", err)
		respondWithError(w, http.StatusUnauthorized, "invalid authorization", err)
		return
	}	

	type parameters struct {
		Email		string
		Password	string
	}

	decoder := json.NewDecoder(req.Body)
	params := parameters{}
	err = decoder.Decode(&params)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't decode parameters", err)
		return
	}
	
	userID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "couldn't decode parameters", err)
		return
	}

	hash, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not hash password", err)
		return
	}

	responseParams := database.UpdateUserLoginParams{
		Email: params.Email,
		HashedPassword: hash,
		ID: userID,
		UpdatedAt: time.Now(),
	}

	updatedUser, err := cfg.db.UpdateUserLogin(context.Background(), responseParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not update user info", err)
		return	
	}
	
	respondWithJSON(w, http.StatusOK, User{
		ID: updatedUser.ID,	
		Email: updatedUser.Email,
		CreatedAt: updatedUser.CreatedAt,
		UpdatedAt: updatedUser.UpdatedAt,
		ChirpyRed: updatedUser.IsChirpyRed,
	})
}

func (cfg *apiConfig) handlerDeleteChirp(w http.ResponseWriter, req *http.Request){

	token, err := auth.GetBearerToken(req.Header)	
	if err != nil {
		log.Println("could not parse authorization token ", err)
		respondWithError(w, http.StatusUnauthorized, "invalid authorization", err)
		return
	}

	chirpIDString := req.PathValue("chirpID")
	chirpID, err := uuid.Parse(chirpIDString)	
	if chirpIDString == "" || err != nil{
		respondWithError(w, http.StatusNotFound, "could not extract chirp id from url", err)
		return
	}
	
	userID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		respondWithError(w, http.StatusForbidden, "invalid token", err)
		return
	}

	chirp, err := cfg.db.GetOneChirp(context.Background(), chirpID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "chirp does not exist", err)
	}
	
	if chirp.UserID != userID {
		respondWithError(w, http.StatusForbidden, "invalid token", err)
		return
	}

	err = cfg.db.DeleteChirp(context.Background(), chirpID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to delete chirp", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (cfg * apiConfig) handlerUpgradeUserRed(w http.ResponseWriter, req *http.Request) {
	
	type requestParameters struct {
		Event string `json:"event"`
		Data  struct {
			UserID uuid.UUID `json:"user_id"`
		} `json:"data"`
	}

	decoder := json.NewDecoder(req.Body)
	params := requestParameters{}
	err := decoder.Decode(&params)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not decode json", err)
		return 
	}
	
		
	apiKey, err := auth.GetAPIKey(req.Header)
	if err != nil || apiKey != cfg.polka_key{
		respondWithError(w, http.StatusUnauthorized, "invalid authorization", err)
		return
	}

	log.Println("Webhook with event ", params.Event)
	if params.Event != "user.upgraded" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	
	log.Println("called upgrade on: ", params.Data.UserID)
	err = cfg.db.UpgradeChirpyRed(context.Background(), params.Data.UserID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "could not upgrade user", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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

	secret := os.Getenv("SECRET")
	polka_key := os.Getenv("POLKA_KEY")
	apiCfg := &apiConfig{
		fileServerHits: atomic.Int32{},
		db: dbQueries,
		platform: platform,
		secret: secret,
		polka_key: polka_key,
	}

	handler := http.StripPrefix("/app", http.FileServer(http.Dir(root)))
	mux := http.NewServeMux()	
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))
		
	mux.HandleFunc("GET /api/healthz", func (w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	})

	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset) 
	mux.HandleFunc("POST /api/revoke", apiCfg.handlerRevoke) 
	mux.HandleFunc("POST /api/users", apiCfg.handlerCreateUser) 
	mux.HandleFunc("POST /api/chirps", apiCfg.handlerCreateChirp) 
	mux.HandleFunc("POST /api/login", apiCfg.handlerUserLogin) 
	mux.HandleFunc("POST /api/refresh", apiCfg.handlerRefresh) 
	mux.HandleFunc("POST /api/polka/webhooks", apiCfg.handlerUpgradeUserRed)

	mux.HandleFunc("GET /api/chirps", apiCfg.handlerGetChirps) 
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.handlerGetChirp) 
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics) 

	mux.HandleFunc("PUT /api/users", apiCfg.handlerUpdateUserInfo)

	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.handlerDeleteChirp)

	srv := &http.Server{
		Addr: ":" + port,
		Handler: mux,
	}
	
	fmt.Println("Serving on port", port)
	log.Fatal(srv.ListenAndServe())
}
