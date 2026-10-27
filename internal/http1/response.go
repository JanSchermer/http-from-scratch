package http1

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type Response struct {
	conn          net.Conn
	header        http.Header
	headerWritten bool
	version       string
	status        int
	data          []byte
}

func NewResponse(conn net.Conn) *Response {
	return &Response{conn, http.Header{}, false, "1.1", 200, []byte{}}
}

func (r *Response) Header() http.Header {
	return r.header
}

func (r *Response) Write(data []byte) (n int, err error) {
	if !r.headerWritten {
		if r.header.Get("Content-Type") == "" {
			detectContentSlice := data[0:min(len(data), 512)]
			r.header.Set("Content-Type", http.DetectContentType(detectContentSlice))
		}
		r.WriteHeader(http.StatusOK)
	}
	r.data = append(r.data, data...)
	return len(data), nil
}

func (r *Response) WriteHeader(status int) {
	r.headerWritten = true
	r.status = status
}

func (r *Response) Send() {
	if r.header.Get("Content-Length") == "" &&
		r.status != http.StatusNoContent &&
		r.status != http.StatusNotModified &&
		r.status > 199 {
		r.header.Set("Content-Length", strconv.Itoa(len(r.data)))
	}
	statusDescription := fmt.Sprintf("%d %s", r.status, http.StatusText(r.status))
	_, _ = r.conn.Write([]byte("HTTP/" + r.version + " " + statusDescription + "\r\n"))
	for k, v := range r.header {
		_, _ = r.conn.Write([]byte(k + ": " + strings.Join(v, ",") + "\r\n"))
	}
	_, _ = r.conn.Write([]byte("\r\n"))
	_, _ = r.conn.Write(r.data)
}
