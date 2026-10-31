package main

import (
	"log"
	"net/http"

	"github.com/ben-anass/DouzTach/server/api/routes"
)

func main() {
	router := routes.NewRouter()

	port := ":8080"

	log.Println("DouzTach API starting on http://localhost" + port)

	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatal(err)
	}
}
