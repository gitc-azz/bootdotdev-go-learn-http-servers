package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"sync/atomic"
	"time"

	"github.com/gitc-azz/bootdotdev-go-learn-http-servers/internal/auth"
	"github.com/gitc-azz/bootdotdev-go-learn-http-servers/internal/database"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/joho/godotenv"

	_ "github.com/lib/pq"
)

func main() {
	// load the '.env' file and make its content available as environment variable
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
		jwtSecret:       os.Getenv("JWT_SECRET"),
		polkaKey:        os.Getenv("POLKA_KEY"),
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
	server_handler.HandleFunc("DELETE /api/chirps/{id}", state.handlerDeleteChirp)
	server_handler.HandleFunc("POST /api/login", state.handlerPostLogin)
	server_handler.HandleFunc("POST /api/refresh", state.handlerPostRefresh)
	server_handler.HandleFunc("POST /api/revoke", state.handlerPostRevoke)
	server_handler.HandleFunc("POST /api/users", state.handlerPostUsers)
	server_handler.HandleFunc("PUT /api/users", state.handlerPutUsers)
	server_handler.HandleFunc("POST /api/polka/webhooks", state.handlerPostPolkaWebHook)

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

func (self *apiConfig) chirpsWithOptionalAuthorId(
	req *http.Request,
	authorId string,
) ([]database.Chirp, error) {

	if authorId == "" {
		return self.dbQueries.Chirps(req.Context())
	}

	id, err := uuid.Parse(authorId)
	if err != nil {
		return []database.Chirp{}, nil
	}

	return self.dbQueries.ChirpsOf(req.Context(), id)
}

func (self *apiConfig) handlerGetChirps(resp http.ResponseWriter, req *http.Request) {
	var chirps []database.Chirp
	var err error

	authorId := req.URL.Query().Get("author_id")
	chirps, err = self.chirpsWithOptionalAuthorId(req, authorId)

	if err != nil {
		errMsg := fmt.Sprintf("failed to fetch chirps from db -> %v", err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))

		return
	}

	isSort := req.URL.Query().Get("sort")
	if isSort == "desc" {
		sort.Slice(chirps, func(i, j int) bool {
			return chirps[i].CreatedAt.Compare(chirps[j].CreatedAt) == 1
		})
	} else {
		sort.Slice(chirps, func(i, j int) bool {
			return chirps[i].CreatedAt.Compare(chirps[j].CreatedAt) == -1
		})
	}

	httpRespondJson(resp, chirps)
}

func (self *apiConfig) handlerChirps(resp http.ResponseWriter, req *http.Request) {
	chirp, err := validate_chirp(resp, req)
	if err != nil {
		return
	}

	// must be authorized to post a chirp
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusUnauthorized, []byte(err.Error()))

		return
	}
	userId, err := auth.ValidateJWT(token, self.jwtSecret)
	if err != nil {
		errMsg := fmt.Errorf("failed to validate JWT token: %v", err).Error()
		httpRespond(resp, "text/plain", http.StatusUnauthorized, []byte(errMsg))

		return
	}

	cleanedChirp := censorship(chirp)

	insertedChirp, err := self.dbQueries.CreateChirp(req.Context(), database.CreateChirpParams{
		Body:   cleanedChirp.Body,
		UserID: userId,
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

func (self *apiConfig) handlerDeleteChirp(resp http.ResponseWriter, req *http.Request) {
	accessToken, err := auth.GetBearerToken(req.Header)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusUnauthorized, []byte(err.Error()))

		return
	}

	userId, err := auth.ValidateJWT(accessToken, self.jwtSecret)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusUnauthorized, []byte(err.Error()))

		return
	}

	idRaw := req.PathValue("id")
	if idRaw == "" {
		errMsg := "url path chirpId is empty"
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))

		return
	}

	id, err := uuid.Parse(idRaw)
	if err != nil {
		errMsg := "chirpId is ill formed"
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))

		return
	}

	chirp, err := self.dbQueries.Chirp(req.Context(), id)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusNotFound, []byte(err.Error()))

		return
	}

	if chirp.UserID != userId {
		errMsg := "not your chirp go away"
		httpRespond(resp, "text/plain", http.StatusForbidden, []byte(errMsg))

		return
	}

	err = self.dbQueries.DeleteChirp(req.Context(), id)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusInternalServerError, []byte(err.Error()))

		return
	}

	httpRespond(resp, "text/plain", http.StatusNoContent, []byte{})
}

func handlerHealthz(resp http.ResponseWriter, req *http.Request) {
	httpRespond(resp, "text/plain; charset=utf-8", 200, []byte("OK"))
}

type apiConfig struct {
	fileServersHits atomic.Int32
	dbQueries       *database.Queries
	isDevPlatform   bool
	jwtSecret       string
	polkaKey        string
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

	token, err := auth.MakeJWT(user.ID, self.jwtSecret, time.Hour)
	if err != nil {
		httpRespond(resp, "test/plain", http.StatusInternalServerError, []byte(err.Error()))

		return
	}

	// keep a trace of that login in our database
	// use of refresh token valid for the next 60 days
	refreshToken := auth.MakeRefreshToken()
	_, err = self.dbQueries.CreateRefreshToken(
		req.Context(),
		database.CreateRefreshTokenParams{
			Token:     refreshToken,
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(time.Hour * 24 * 60),
		})
	if err != nil {
		errMsg :=
			fmt.Errorf("failed to create an database entry refresh token -> %v", err).Error()
		httpRespond(resp, "text/plain", http.StatusInternalServerError, []byte(errMsg))

		return
	}

	userWithJWT := struct {
		ID           uuid.UUID `json:"id"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		Email        string    `json:"email"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
		IsChirpyRed  bool      `json:"is_chirpy_red"`
	}{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        token,
		RefreshToken: refreshToken,
		IsChirpyRed:  user.IsChirpyRed,
	}

	to_send, err := json.Marshal(userWithJWT)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusInternalServerError, []byte(err.Error()))

		return
	}

	httpRespond(resp, "application/json", http.StatusOK, to_send)
}

func (self *apiConfig) handlerPostRefresh(resp http.ResponseWriter, req *http.Request) {
	refreshToken, err := auth.GetBearerToken(req.Header)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(err.Error()))

		return
	}

	rtDatabase, err := self.dbQueries.GetRefreshToken(req.Context(), refreshToken)
	isUnauthorized := err != nil ||
		rtDatabase.RevokedAt.Valid ||
		rtDatabase.ExpiresAt.Compare(time.Now()) == -1
	if isUnauthorized {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		} else if rtDatabase.RevokedAt.Valid {
			errMsg = "refresh token revoked"
		} else {
			errMsg = "refresh token expired"
		}
		httpRespond(resp, "text/plain", http.StatusUnauthorized, []byte(errMsg))

		return
	}

	accessToken, err := auth.MakeJWT(rtDatabase.UserID, self.jwtSecret, time.Hour)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusInternalServerError, []byte(err.Error()))

		return
	}

	retToMarshal := struct {
		Token string `json:"token"`
	}{
		Token: accessToken,
	}

	retToSend, err := json.Marshal(retToMarshal)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusInternalServerError, []byte(err.Error()))

		return
	}

	httpRespond(resp, "application/json", http.StatusOK, retToSend)
}

func (self *apiConfig) handlerPostRevoke(resp http.ResponseWriter, req *http.Request) {
	refreshToken, err := auth.GetBearerToken(req.Header)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(err.Error()))

		return
	}

	err = self.dbQueries.RevokeRefreshToken(req.Context(), refreshToken)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusNotModified, []byte(err.Error()))

		return
	}

	httpRespond(resp, "text/plain", http.StatusNoContent, []byte(""))
}

func (self *apiConfig) handlerPutUsers(resp http.ResponseWriter, req *http.Request) {
	accessToken, err := auth.GetBearerToken(req.Header)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusUnauthorized, []byte(err.Error()))

		return
	}

	userId, err := auth.ValidateJWT(accessToken, self.jwtSecret)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusUnauthorized, []byte(err.Error()))

		return
	}

	input := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{}

	decoder := json.NewDecoder(req.Body)
	defer req.Body.Close()

	if err = decoder.Decode(&input); err != nil {
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(err.Error()))

		return
	}

	hashedPassword, err := auth.HashPassword(input.Password)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusInternalServerError, []byte(err.Error()))

		return
	}

	err = self.dbQueries.UpdateUsers(
		req.Context(),
		database.UpdateUsersParams{
			ID:             userId,
			Email:          input.Email,
			HashedPassword: hashedPassword,
		},
	)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusInternalServerError, []byte(err.Error()))

		return
	}

	retToMarshal := struct {
		Email string `json:"email"`
	}{
		Email: input.Email,
	}

	retToSend, err := json.Marshal(retToMarshal)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusInternalServerError, []byte(err.Error()))

		return
	}

	httpRespond(resp, "application/json", http.StatusOK, retToSend)
}

func (self *apiConfig) handlerPostPolkaWebHook(resp http.ResponseWriter, req *http.Request) {
	apiKey, err := auth.GetAPIKey(req.Header)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusUnauthorized, []byte(err.Error()))

		return
	}
	if apiKey != self.polkaKey {
		errMsg := "wrong api key"
		httpRespond(resp, "text/plain", http.StatusUnauthorized, []byte(errMsg))

		return
	}
	input := struct {
		Event string `json:"event"`
		Data  struct {
			UserId uuid.UUID `json:"user_id"`
		} `json:"data"`
	}{}

	decoder := json.NewDecoder(req.Body)
	defer req.Body.Close()

	if err := decoder.Decode(&input); err != nil {
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(err.Error()))

		return
	}

	if input.Event != "user.upgraded" {
		httpRespond(resp, "text/plain", http.StatusNoContent, []byte{})

		return
	}

	err = self.dbQueries.UpgradeUserToRed(req.Context(), input.Data.UserId)
	if err != nil {
		httpRespond(resp, "text/plain", http.StatusNotFound, []byte(err.Error()))

		return
	}

	httpRespond(resp, "text/plain", http.StatusNoContent, []byte{})
}
