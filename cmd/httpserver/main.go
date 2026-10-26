package main

import (
	"fmt"
	"httpServer/internal/http1"
	"io"
	"net/http"
)

const addr = ":8080"

func main() {
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello")
	})

	http.HandleFunc("/echoBody", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(string(body))
		_, _ = w.Write(body)
	})

	if err := http1.ListenAndServe(addr, nil); err != nil {
		panic(err)
	}
}
