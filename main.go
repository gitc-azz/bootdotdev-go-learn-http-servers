package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/gitc-azz/bootdotdev-go-learn-http-servers/internal/database"
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
	server_handler.HandleFunc("POST /api/validate_chirp", handlerValidateChirps)
	server_handler.HandleFunc("POST /api/users", state.handlerUsers)

	server := http.Server{
		Handler: server_handler,
		Addr:    ":8080",
	}

	err = server.ListenAndServe()

	if err != nil {
		log.Fatal("failed to listen and serve")
	}
}

func handlerValidateChirps(resp http.ResponseWriter, req *http.Request) {
	chirp, err := validate_chirp(resp, req)
	if err != nil {
		return
	}

	cleaned := censorship(chirp)

	toSend, err := json.Marshal(cleaned)
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	httpRespond(resp, "application/json; charset=utf-8", http.StatusOK, toSend)
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

func (self *apiConfig) handlerUsers(resp http.ResponseWriter, req *http.Request) {
	usersJson := struct {
		Email string `json:"email"`
	}{}

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&usersJson)
	if err != nil {
		errMsg := fmt.Sprintf(`error decoding json %v, expect: {"email":"..."}`, err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))
	}

	user, err := self.dbQueries.CreateUser(req.Context(), usersJson.Email)
	if err != nil {
		errMsg := fmt.Sprintf("database create user failed -> %v", err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))
	}

	bytes, err := json.Marshal(user)
	if err != nil {
		errMsg := fmt.Sprintf("failed to marshal created user from db -> %v", err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))
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
