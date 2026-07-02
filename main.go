package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/arussellsaw/news/dao"
	"github.com/arussellsaw/news/handler"
	"github.com/arussellsaw/news/idgen"
)

func main() {
	ctx := context.Background()

	err := idgen.Init(ctx)
	if err != nil {
		log.Fatalf("Error initialising idgen: %v", err)
	}

	err = dao.Init(ctx)
	if err != nil {
		log.Fatalf("Error initialising dao: %v", err)
	}
	defer dao.Close()

	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	log.Printf("Ready, listening on %s", addr)
	if err := http.ListenAndServe(addr, handler.Init(ctx)); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
