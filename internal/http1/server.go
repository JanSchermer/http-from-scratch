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
			fmt.Printf(err.Error())
		}
	}
}

func HandleConnection(conn net.Conn, mux http.Handler) error {
	reader := bufio.NewReader(conn)
	request, err := ParseRequest(reader)
	response := NewResponse(conn)
	defer func(conn net.Conn) {
		err = conn.Close()
		if err != nil {
			_ = fmt.Errorf("close connection: %w", err)
		}
	}(conn)
	if err != nil {
		response.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("parse request: %w", err)
	}
	mux.ServeHTTP(response, request)
	return nil
}
