// Command server serves the tasks API.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"example.com/tasks/internal/api"
	"example.com/tasks/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.NewHandler(store.NewMemory()),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("listening on %s", *addr)
	log.Fatal(srv.ListenAndServe())
}
