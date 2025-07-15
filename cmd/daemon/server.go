package main

import (
	"log"
	"net/http"
)

func Start() error {
	http.HandleFunc("/nextcore", nextCoreHandler)

	log.Println("✅ Daemon is running on :8080")
	return http.ListenAndServe(":8080", nil)
}
