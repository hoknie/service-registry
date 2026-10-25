package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(prometheus.Gatherers{m.events, m.snapshot}, promhttp.HandlerOpts{})
}
