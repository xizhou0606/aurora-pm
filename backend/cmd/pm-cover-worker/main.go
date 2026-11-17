package main

import (
	"log"

	"github.com/xizhou0606/aurora-pm/backend/internal/app/coverworker"
)

func main() {
	if err := coverworker.Run(); err != nil {
		log.Fatal(err)
	}
}
