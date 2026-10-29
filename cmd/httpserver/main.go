package main

import (
	"httpServer/internal/http1"
	"httpServer/internal/monitoring"
)

const addr = ":8080"

func main() {
	monitor := monitoring.NewMonitor()
	monitoring.StartTransmitting()

	if err := http1.ListenAndServe(addr, monitor); err != nil {
		panic(err)
	}
}
