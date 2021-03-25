# binance-home-task

## Tasks

1. Print the top 5 symbols with quote asset `BTC` and the highest volume over the last 24 hours in descending order.
2. Print the top 5 symbols with quote asset `USDT` and the highest number of trades over the last 24 hours in descending order.
3. Using the symbols from Q1, what is the total notional value of the top 200 bids and asks currently on each order book?
4. What is the price spread for each of the symbols from Q2?
5. Every 10 seconds print the result of Q4 and the absolute delta from the previous value for each symbol.
6. Make the output of Q5 accessible by querying http://localhost:8080/metrics using the Prometheus Metrics format.

## Quick Start

```sh
$ go build -o out/binancehometask && ./out/binancehometask
```

1. Navigate to http://localhost:8080 to see the output for tasks Q1, Q2, Q3, Q4

1. Check the console output to see Q5

1. Navigate to http://localhost:8080/metrics to see Q6, display metrics updated 
with the interval defined in Q5 regardless of the Prometheus scraping interval.


## Implementation Details

### Project Structure

```sh
.
├── background.go     # background worker which reports spreads data
├── client.go         # binance api client implementation
├── health.go         # health checks
├── index.go          # index web page action
├── index.html        # index web page template
├── logging.go        # logging configuration and middleware
├── main.go           # entry point and server startup
├── metrics.go        # prometheus metric collector
├── model.go          # binance api models
├── service.go        # market data service which calls api
├── sorting.go        # utility sorting functions
└── tracing.go        # tracing middleware
```

### Client Implementation

_NOTE: When performing API calls need to maintain the used weight,
and the real world example should be built with sockets._

Exchange info is fetched and cached on the client, and acts
as a listed symbols metadata.

Ticker change statistics is very expensive method when symbol
arg is omitted, thus the API response is cached on a 
client for very short period to allow reuse of the fetched data during this time frame.

Order book method is used to calculate symbol volume (top 200 asks and bids) as well as to get the ask-bid spread and absolute 
delta between current and previous spread values.

The decimal type is used for fields returned by API to not loose 
a precision with float type.

When `debug` level logging enabled, it API call operation reports
`x-mbx-used-weight` used.

### Market Data Service

The service wraps the logic to interact with client calling remote API. 

It also uses goroutines to parallel the client calls to fetch the
order book for different symbols.

Sorting is done by implementing the `sort` interface methods Less, Len and Swap.
With few more utility structures (`sorting.go`)

### Background Worker

The background service is started from the main thread, and maintains its
background task configured to run by cron (`gocron` lib).

In the background service the fetched symbols with top number of trades 
(targets to calculate the spreads) are cached for the reasonable amount of time.

The calculated spreads data is outputted to the console (log output is configured
in `logging.go` and can vary when targeting the prod env).

Also the cache with auto-expire is utilised as a communication channel
to set spread metrics which should be collected by Prometheus.
So regardless of the Prometheus scraping interval no extra calls would be
performed to the remote API.

### Metrics

The application metrics are exposed in Prometheus format at `/metrics` endpoint.

The spread value and spread delta metrics are implemented with custom Prometheus
collector to extend the Gauges with exta labels such as `symbol` and `sign` for 
the absolute delta value.

### Configuration Parameters

The application accepts configuration parameters as the command line args.
In advanced example the config usually would be supplied via environment variables
(not implemented for simplicity).

```
$ ./out/binancehometask -h
Usage of ./out/binancehometask:
  -api-url string
        public Rest API for Binance (default "https://api.binance.com")
  -listen-addres string
        server listen address (default ":8080")
  -log-level string
        minimum logging level (default "info")
```

### Health Checks

The application has liveness `/live` and readiness `ready` probe handlers.

A failed liveness check indicates that the app is unhealthy, and the app should be
destroyed or restarted.

A failed readiness check indicates that the app is currently unable to serve requests,
because of an upstream or some transient failure, and the app should no longer receive
requests.

### Logging & Tracing Middlewares

The application is configured to log every HTTP request.

If request identifier is provided using header `X-Request-Id` it will be
logged as well, otherwise unique string is generated.


## Gotchas

### No graceful shutdown

The application doesn't use the graceful shutdown for simplicity.

### Lack of comments to functions and types

Self-documented code is more pleasant to read than scanning the code pages 
with pure variable names and huge comments.
