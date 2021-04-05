package main

import (
	"strconv"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

const SPREAD_METRICS_KEY = "spredMetrics"

var MetricsCache = cache.New(time.Duration(10)*time.Second, time.Duration(10)*time.Second)

func init() {
	prometheus.MustRegister(newMetricsCollector())
}

type SpreadMetric struct {
	spread *Spread
	delta  decimal.Decimal
}

type metricsCollector struct {
	prometheus.Collector
	spreadMetric      *prometheus.Desc
	spreadDeltaMetric *prometheus.Desc
}

func newMetricsCollector() *metricsCollector {
	return &metricsCollector{
		spreadMetric: prometheus.NewDesc(
			"spread_value",
			"Bid-ask spread of the symbol",
			[]string{"symbol"}, nil,
		),
		spreadDeltaMetric: prometheus.NewDesc(
			"spread_delta",
			"Absolute delta from the previous spread value with sign label",
			[]string{"symbol", "sign"}, nil,
		),
	}
}

func (c *metricsCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debug("Collect Prometheus metrics from spread metrics cache")
	var metrics map[string]*SpreadMetric
	if x, found := MetricsCache.Get(SPREAD_METRICS_KEY); found {
		metrics = x.(map[string]*SpreadMetric)
		for _, sm := range metrics {
			c.setSpreadMetrics(sm, ch)
		}
	}
}

func (c *metricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.spreadMetric
	ch <- c.spreadDeltaMetric
}

func (c *metricsCollector) setSpreadMetrics(sm *SpreadMetric, ch chan<- prometheus.Metric) {
	value, _ := sm.spread.Value.Float64()
	ch <- prometheus.MustNewConstMetric(c.spreadMetric, prometheus.GaugeValue, value, sm.spread.Symbol)

	dvalue, _ := sm.delta.Abs().Float64()
	sign := strconv.Itoa(sm.delta.Sign())
	ch <- prometheus.MustNewConstMetric(c.spreadDeltaMetric, prometheus.GaugeValue, dvalue, sm.spread.Symbol, sign)
}
