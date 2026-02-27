package ws

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var wsConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "tableforge_ws_connections_active",
	Help: "Number of active WebSocket connections.",
})
