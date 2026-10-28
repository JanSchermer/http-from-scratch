package http1

import "net/http"

type Handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

type HandlerFunc func(http.ResponseWriter, *http.Request)

type anonymousHandler struct {
	f HandlerFunc
}

func (h anonymousHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.f(w, r)
}

func newAnonymousHandler(f HandlerFunc) Handler {
	return anonymousHandler{f: f}
}
