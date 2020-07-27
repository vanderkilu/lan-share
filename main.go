package main

import (
	"log"

	"./server"
)

func main() {

	config := server.Config{
		Port: "8080",
	}
	s, err := server.NewServer(config)
	if err != nil {
		log.Fatal(err)
	}
	s.Welcome()
	s.Wait()
}
