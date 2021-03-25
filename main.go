package main

import (
	"flag"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

type middleware func(http.Handler) http.Handler
type middlewares []middleware

type controller struct {
	logger        *log.Logger
	nextRequestID func() string
}

var (
	apiBaseUrl    string
	listenAddress string
	logLevel      string
)

func main() {

	flag.StringVar(&apiBaseUrl, "api-base-url", "https://api.binance.com", "public Rest API for Binance")
	flag.StringVar(&listenAddress, "listen-addres", ":8080", "server listen address")
	flag.StringVar(&logLevel, "log-level", "info", "minimum logging level")
	flag.Parse()

	l, err := log.ParseLevel(logLevel)
	if err != nil {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(l)
	}

	c := &controller{logger: log.New(), nextRequestID: func() string { return strconv.FormatInt(time.Now().UnixNano(), 36) }}

	router := http.NewServeMux()
	router.HandleFunc("/", c.index)

	router.Handle("/metrics", promhttp.Handler())

	health := healtcheck()
	router.HandleFunc("/live", health.LiveEndpoint)
	router.HandleFunc("/ready", health.ReadyEndpoint)

	client := NewApiClient()
	service := NewMarketDataService(&client)
	background := NewBackgroundService(&service)
	go background.Start()

	log.WithField("listen-addres", listenAddress).Info("Starting HTTP server")
	log.Fatal(http.ListenAndServe(listenAddress, (middlewares{c.tracing, c.logging}).apply(router)))
}

func (mws middlewares) apply(hdlr http.Handler) http.Handler {
	if len(mws) == 0 {
		return hdlr
	}
	return mws[1:].apply(mws[0](hdlr))
}
