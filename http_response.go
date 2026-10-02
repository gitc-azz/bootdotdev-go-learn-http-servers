package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func httpRespond(resp http.ResponseWriter, contentType string, statusCode int, toWrite []byte) {
	resp.Header().Set("Content-Type", contentType)
	resp.WriteHeader(statusCode)
	resp.Write(toWrite)
}

func httpRespondJson(resp http.ResponseWriter, data any) {
	toSend, err := json.Marshal(data)
	if err != nil {
		errMsg := fmt.Sprintf("failed to marshal -> %v", err)
		httpRespond(resp, "text/plain", http.StatusBadRequest, []byte(errMsg))

		return
	}

	httpRespond(resp, "application/json", http.StatusOK, toSend)
}
