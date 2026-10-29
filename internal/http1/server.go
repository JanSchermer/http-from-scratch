package http1

import (
	"bufio"
	"fmt"
	"httpServer/internal/monitoring"
	"net"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

func ListenAndServe(addr string, monitor *monitoring.Monitor) error {
	//tlsConfig := GetTLSConfig()
	//listener, err := tls.Listen("tcp", addr, tlsConfig)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()

	handler := NewServeMux()

	InitUsersService(handler)
	handler.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics.WritePrometheus(w, true)
	})

	for {
		var conn net.Conn
		conn, err = listener.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}
		err = HandleConnection(conn, handler, monitor)
		if err != nil {
			fmt.Println(err.Error())
		}
	}
}

func HandleConnection(conn net.Conn, mux Handler, monitor *monitoring.Monitor) error {
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
		monitor.IncRequest()
		start := time.Now()
		if err != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Send()
			monitor.IncRequestError()
			return nil
		}
		if request == nil {
			return nil // Client disconnected
		}
		if request.Proto != "HTTP/1.1" && request.Proto != "HTTP/1.0" {
			response.WriteHeader(http.StatusHTTPVersionNotSupported)
			response.Send()
			monitor.IncRequestError()
			return nil
		}
		mux.ServeHTTP(response, request)
		response.Send()
		monitor.AddRequestDuration(time.Since(start))
		if (request.ProtoMinor == 0 && (len(request.Header["Connection"]) == 0 || request.Header["Connection"][0] != "keep-alive")) ||
			(len(request.Header["Connection"]) > 0 && request.Header["Connection"][0] == "close") {
			return nil // Disconnect by default
		}
	}
}
