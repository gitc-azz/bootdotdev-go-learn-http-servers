package main

import (
	"net/http"
)

func httpRespond(resp http.ResponseWriter, contentType string, statusCode int, toWrite []byte) {
	resp.Header().Set("Content-Type", contentType)
	resp.WriteHeader(statusCode)
	resp.Write(toWrite)
}
