package http1

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

type Response struct {
	conn       net.Conn
	header     http.Header
	headerSend bool
	version    string
}

func NewResponse(conn net.Conn) *Response {
	return &Response{conn, http.Header{}, false, "1.1"}
}

func (r *Response) Header() http.Header {
	return r.header
}

func (r *Response) Write(data []byte) (n int, err error) {
	if !r.headerSend {
		if r.header.Get("Content-Type") == "" {
			detectContentSlice := data[0:min(len(data), 512)]
			r.header.Set("Content-Type", http.DetectContentType(detectContentSlice))
		}
		if r.header.Get("Content-Length") == "" {
			r.header.Set("Content-Length", fmt.Sprint(len(data)))
		}
		r.WriteHeader(http.StatusOK)
	}
	return r.conn.Write(data)
}

func (r *Response) WriteHeader(statusCode int) {
	r.headerSend = true
	statusDescription := fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode))
	_, _ = r.conn.Write([]byte("HTTP/" + r.version + " " + statusDescription + "\r\n"))
	for k, v := range r.header {
		_, _ = r.conn.Write([]byte(k + ": " + strings.Join(v, ",") + "\r\n"))
	}
	_, _ = r.conn.Write([]byte("\r\n"))
}
