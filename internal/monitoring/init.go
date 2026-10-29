package monitoring

import (
	"time"

	"github.com/VictoriaMetrics/metrics"
)

type Monitor struct {
	requestTotal      *metrics.Counter
	requestErrorTotal *metrics.Counter
	requestDuration   *metrics.Summary
}

func NewMonitor() *Monitor {
	return &Monitor{
		requestTotal:      metrics.NewCounter("request_total"),
		requestErrorTotal: metrics.NewCounter("request_error_total"),
		requestDuration:   metrics.NewSummary(`request_duration_seconds{path="/echo"}`),
	}
}

func (m *Monitor) IncRequest() {
	m.requestTotal.Inc()
}

func (m *Monitor) IncRequestError() {
	m.requestErrorTotal.Inc()
}

func (m *Monitor) AddRequestDuration(duration time.Duration) {
	m.requestDuration.Update(float64(duration))
}

func StartTransmitting() {
	err := metrics.InitPush("http://vmsingle-vm-victoria-metrics-k8s-stack:8428/api/v1/import/prometheus", 5*time.Second, `instance="local"`, true)
	if err != nil {
		panic(err)
	}
}
