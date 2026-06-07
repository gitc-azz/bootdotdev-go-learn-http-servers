package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

func main() {
	state := &apiConfig{fileServersHits: atomic.Int32{}}
	server_handler := http.NewServeMux()
	server_handler.Handle("/app/",
		http.StripPrefix("/app",
			state.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	server_handler.HandleFunc("GET /api/healthz", handlerHealthz)
	server_handler.HandleFunc("GET /admin/metrics", state.handlerMetrics)
	server_handler.HandleFunc("POST /admin/reset", state.handlerReset)
	server_handler.HandleFunc("POST /api/validate_chirp", handlerValidateChirps)

	server := http.Server{
		Handler: server_handler,
		Addr:    ":8080",
	}

	err := server.ListenAndServe()

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
}

func (self *apiConfig) inc() {
	self.fileServersHits.Add(1)
}

func (self *apiConfig) handlerReset(http.ResponseWriter, *http.Request) {
	self.fileServersHits.Store(0)
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
