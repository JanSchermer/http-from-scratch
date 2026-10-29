package http1

import (
	"encoding/json"
	"io"
	"net/http"
)

type User struct {
	Name string `json:"name"`
}

func InitUsersService(mux *ServeMux) {
	users := make([]User, 0, 10)
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			defer func(Body io.ReadCloser) {
				_ = Body.Close()
			}(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			user := User{}
			if err := json.Unmarshal(body, &user); err != nil {
				w.WriteHeader(http.StatusBadRequest)
			}
			users = append(users, user)
			w.WriteHeader(http.StatusCreated)
		} else if r.Method == http.MethodGet {
			bytes, err := json.Marshal(users)
			if err != nil {
				panic(err)
			}
			_, _ = w.Write(bytes)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}
