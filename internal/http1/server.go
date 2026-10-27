package http1

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
)

func ListenAndServe(addr string, handler http.Handler) error {
	tlsConfig := GetTLSConfig()
	listener, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()

	if handler == nil {
		handler = http.DefaultServeMux
	}

	for {
		var conn net.Conn
		conn, err = listener.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}
		err = HandleConnection(conn, handler)
		if err != nil {
			fmt.Println(err.Error())
		}
	}
}

func HandleConnection(conn net.Conn, mux http.Handler) error {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			_ = fmt.Errorf("close connection: %w", err)
		}
	}(conn)
	reader := bufio.NewReader(conn)
	for {
		request, err := ParseRequest(reader)
		response := NewResponse(conn)
		if err != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Send()
			return nil
		}
		if request == nil {
			return nil // Client disconnected
		}
		if request.Proto != "HTTP/1.1" && request.Proto != "HTTP/1.0" {
			response.WriteHeader(http.StatusHTTPVersionNotSupported)
			response.Send()
			return nil
		}
		mux.ServeHTTP(response, request)
		response.Send()
		if (request.ProtoMinor == 0 && (len(request.Header["Connection"]) == 0 || request.Header["Connection"][0] != "keep-alive")) ||
			(len(request.Header["Connection"]) > 0 && request.Header["Connection"][0] == "close") {
			return nil // Disconnect by default
		}
	}
}
