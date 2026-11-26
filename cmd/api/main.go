package main

import (
	"expense-tracker/internal/httpapi"
	"log"
	"net/http"
	"time"
)

func main() {
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           httpapi.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("Listeneing on %s", srv.Addr)

}
