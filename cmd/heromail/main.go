package main

import (
	"log"
	"net/http"
	"os"
	"time"

	httpapi "github.com/ljunn/heromail/internal/http"
	"github.com/ljunn/heromail/internal/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	st := store.New()
	server := httpapi.NewServer(st)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			st.ReapExpired()
		}
	}()
	log.Printf("HeroMail listening on :%s", port)
	if err := http.ListenAndServe(":"+port, server.Router); err != nil {
		log.Fatal(err)
	}
}
