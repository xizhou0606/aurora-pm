package main

import (
	"log"

	appworker "github.com/xizhou0606/aurora-pm/backend/internal/app/worker"
)

func main() {
	if err := appworker.Run(); err != nil {
		log.Fatal(err)
	}
}
