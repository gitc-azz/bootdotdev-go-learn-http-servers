package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type chirp struct {
	Body   string    `json:"body"`
	UserId uuid.UUID `json:"user_id"`
}

func validate_chirp(resp http.ResponseWriter, req *http.Request) (chirp, error) {
	decoder := json.NewDecoder(req.Body)

	chirp := chirp{}
	if err := decoder.Decode(&chirp); err != nil {
		msg := fmt.Sprintf(`{"error": "failed to read chirp -> %v"`, err)
		httpRespond(resp, "application/json", 400, []byte(msg))
		return chirp, errors.New("stop")
	}

	if len(chirp.Body) > 140 {
		httpRespond(resp, "application/json", 400, []byte(`{"error": "chirp is too long"}`))

		return chirp, errors.New("stop")
	}

	return chirp, nil
}

func censorship(c chirp) chirp {
	words := strings.Split(c.Body, " ")
	forbidden := []string{"kerfuffle", "sharbert", "fornax"}

	ret := c
	ret.Body = ""
	for idx, word := range words {

		for idx, _ := range forbidden {
			if strings.EqualFold(word, forbidden[idx]) {
				word = "****"
			}
		}
		suffix := " "
		if len(words) == idx+1 {
			suffix = ""
		}
		ret.Body += word + suffix

	}

	return ret
}
