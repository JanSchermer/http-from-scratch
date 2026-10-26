package main

import (
	"httpServer/internal/http1"
	"httpServer/internal/users"
)

const addr = ":8080"

func main() {
	users.InitUsersService()

	if err := http1.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}
