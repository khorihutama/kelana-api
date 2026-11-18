package main

import (
	"log"
	"os"

	"github.com/khorihutama/hsi-go-api/internal/config"
	"github.com/khorihutama/hsi-go-api/internal/infra/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if err := database.Migrate(cfg.DSN(), command); err != nil {
		log.Fatalf("migration %s failed: %v", command, err)
	}
	log.Printf("migration %s completed", command)
}
