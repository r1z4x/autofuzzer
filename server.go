package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", loggingMiddleware(handleRequest))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Gelen İstek: %s %s\n", r.Method, r.URL.Path)
		next(w, r)
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Herhangi bir route için 200 OK")
}
