package main

import (
	"log"

	appapi "github.com/xizhou0606/aurora-pm/backend/internal/app/api"
)

func main() {
	if err := appapi.Run(); err != nil {
		log.Fatal(err)
	}
}
