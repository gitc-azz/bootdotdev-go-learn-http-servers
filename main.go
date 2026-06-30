package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/gitc-azz/bootdotdev-go-learn-http-servers/internal/auth"
	"github.com/gitc-azz/bootdotdev-go-learn-http-servers/internal/database"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/joho/godotenv"

	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()

	dbUrl := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		log.Fatalf("sql failed to open as %v -> %v", dbUrl, err)
	}

	state := &apiConfig{
		fileServersHits: atomic.Int32{},
		dbQueries:       database.New(db),
		isDevPlatform:   os.Getenv("PLATFORM") == "DEV",
	}
	server_handler := http.NewServeMux()
	server_handler.Handle("/app/",
		http.StripPrefix("/app",
			state.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	server_handler.HandleFunc("GET /api/healthz", handlerHealthz)
	server_handler.HandleFunc("GET /admin/metrics", state.handlerMetrics)
	server_handler.HandleFunc("POST /admin/reset", state.handlerReset)
	server_handler.HandleFunc("POST /api/chirps", state.handlerChirps)
	server_handler.HandleFunc("GET /api/chirps", state.handlerGetChirps)
	server_handler.HandleFunc("GET /api/chirps/{id}", state.handlerGetChirp)
	server_handler.HandleFunc("POST /api/login", state.handlerPostLogin)
	server_handler.HandleFunc("POST /api/users", state.handlerPostUsers)

	server := http.Server{
		Handler: server_handler,
		Addr:    ":8080",
	}

	err = server.ListenAndServe()

	if err != nil {
		log.Fatalf("failed to listen and serve -> %v", err)
	}
}

func (self *apiConfig) handlerGetChirp(resp http.ResponseWriter, req *http.Request) {
	idRaw := req.PathValue("id")
	if idRaw == "" {
		httpRespond(resp, "text/plain", http.StatusBadRequest,
			[]byte("id of the chirp is empty"))

		return
	}
	id, err := uuid.Parse(idRaw)
	if err != nil {
		errMsg := fmt.Sprintf("invalid uuid {%v} -> %v", idRaw, err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))

		return
	}

	chirp, err := self.dbQueries.Chirp(req.Context(), id)
	if err != nil {
		errMsg := fmt.Sprintf("failed to fetch from db, chirp {%v} -> %v",
			id, err)
		httpRespond(resp, "text/plain", http.StatusNotFound, []byte(errMsg))

		return
	}

	toSend, err := json.Marshal(chirp)
	if err != nil {
		errMsg := fmt.Sprintf("failed to marshal chirp -> %v", err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))

		return
	}

	httpRespond(resp, "application/json", http.StatusOK, toSend)
}

func (self *apiConfig) handlerGetChirps(resp http.ResponseWriter, req *http.Request) {
	chirps, err := self.dbQueries.Chirps(req.Context())
	if err != nil {
		errMsg := fmt.Sprintf("failed to fetch chirps from db -> %v", err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))

		return
	}

	toSend, err := json.Marshal(chirps)
	if err != nil {
		errMsg := fmt.Sprintf("failed to marshal chirps -> %v", err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))

		return
	}

	httpRespond(resp, "application/json", http.StatusOK, toSend)
}

func (self *apiConfig) handlerChirps(resp http.ResponseWriter, req *http.Request) {
	chirp, err := validate_chirp(resp, req)
	if err != nil {
		return
	}

	cleanedChirp := censorship(chirp)

	insertedChirp, err := self.dbQueries.CreateChirp(req.Context(), database.CreateChirpParams{
		Body:   cleanedChirp.Body,
		UserID: cleanedChirp.UserId,
	})
	if err != nil {
		errMsg := fmt.Sprintf("failed to insert chirp -> %v", err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))

		return
	}

	toSend, err := json.Marshal(insertedChirp)
	if err != nil {
		errMsg := fmt.Sprintf("failed to marshal chirp from db -> %v", err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))

		return
	}

	httpRespond(resp, "application/json; charset=utf-8", http.StatusCreated, toSend)
}

func handlerHealthz(resp http.ResponseWriter, req *http.Request) {
	httpRespond(resp, "text/plain; charset=utf-8", 200, []byte("OK"))
}

type apiConfig struct {
	fileServersHits atomic.Int32
	dbQueries       *database.Queries
	isDevPlatform   bool
}

func (self *apiConfig) inc() {
	self.fileServersHits.Add(1)
}

func (self *apiConfig) handlerPostUsers(resp http.ResponseWriter, req *http.Request) {
	usersJson := struct {
		Email    string `json:"email" validate:"required"`
		Password string `json:"password" validate:"required"`
	}{}

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&usersJson)
	if err != nil {
		errMsg := fmt.Sprintf(`error decoding json %v, expect: {"email":"..."}`, err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))

		return
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err = validate.Struct(usersJson); err != nil {
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(err.Error()))

		return
	}

	hashed_password, err := auth.HashPassword(usersJson.Password)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusInternalServerError, []byte(err.Error()))

		return
	}

	user, err := self.dbQueries.CreateUser(req.Context(),
		database.CreateUserParams{
			Email:          usersJson.Email,
			HashedPassword: hashed_password,
		},
	)
	if err != nil {
		errMsg := fmt.Sprintf("database create user failed -> %v", err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))

		return
	}

	bytes, err := json.Marshal(user)
	if err != nil {
		errMsg := fmt.Sprintf("failed to marshal created user from db -> %v", err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))

		return
	}

	httpRespond(resp, "application/json", http.StatusCreated, bytes)
}

func (self *apiConfig) handlerReset(resp http.ResponseWriter, req *http.Request) {
	if !self.isDevPlatform {
		httpRespond(resp, "text/plain", http.StatusForbidden,
			[]byte("Endpoint exclusive to devs"))

		return
	}

	self.fileServersHits.Store(0)
	err := self.dbQueries.EmptyUsers(req.Context())
	if err != nil {
		errMsg := fmt.Sprintf("failed to empty table users -> %v", err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))
	}
}

func (self *apiConfig) handlerMetrics(resp http.ResponseWriter, req *http.Request) {
	msg := fmt.Sprintf(
		`
		<html>
			<body>
				<h1>Welcome, Chirpy Admin</h1>
				<p>Chirpy has been visited %d times!</p>
			</body>
		</html>
		`, self.fileServersHits.Load())

	httpRespond(resp, "text/html", 200, []byte(msg))
}

func (self *apiConfig) middlewareMetricsInc(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		self.inc()
		handler.ServeHTTP(resp, req)
	})
}

func (self *apiConfig) handlerPostLogin(resp http.ResponseWriter, req *http.Request) {
	var logreq struct {
		Password string `json:"password" validate:required`
		Email    string `json:"email" validate:required`
	}

	decoder := json.NewDecoder(req.Body)
	defer req.Body.Close()

	if err := decoder.Decode(&logreq); err != nil {
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(err.Error()))

		return
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(logreq); err != nil {
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(err.Error()))

		return
	}

	user, err := self.dbQueries.UserByEmail(req.Context(), logreq.Email)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusInternalServerError, []byte(err.Error()))

		return
	}

	valid_password, err := auth.CheckPassword(logreq.Password, user.HashedPassword)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusInternalServerError, []byte(err.Error()))

		return
	}
	if !valid_password {
		httpRespond(resp, "text/plain", http.StatusUnauthorized, []byte("wrong credential"))

		return
	}

	user_without_password := database.CreateUserRow{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}

	to_send, err := json.Marshal(user_without_password)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusInternalServerError, []byte(err.Error()))

		return
	}

	httpRespond(resp, "application/json", http.StatusOK, to_send)
}
