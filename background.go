package main

import (
	"sort"
	"time"

	"github.com/jasonlvhit/gocron"
	"github.com/patrickmn/go-cache"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

const (
	TOP_TRADE_COUNT_KEY        = "topTradeCount"
	SPREAD_METRICS_QUOTE_ASSET = "USDT"
)

var topTradeCountCache = cache.New(time.Duration(5)*time.Minute, time.Duration(5)*time.Minute)

type background struct {
	service MarketDataService
	state   map[string]*SpreadMetric
}

type BackgroundService interface {
	Start()
}

func NewBackgroundService(s *MarketDataService) BackgroundService {
	return &background{
		service: *s,
		state:   make(map[string]*SpreadMetric),
	}
}

func (b *background) Start() {
	b.backgroundTask()
	gocron.Every(10).Second().Do(b.backgroundTask)
	<-gocron.Start()
}

func (b *background) backgroundTask() {
	// get top number of trades
	// cached or fetch from api
	var topNumberOfTrades []*SymbolData
	if x, found := topTradeCountCache.Get(TOP_TRADE_COUNT_KEY); found {
		topNumberOfTrades = x.([]*SymbolData)
	} else {
		byTradeCountSort := func(symbols []*SymbolData) {
			sort.Sort(ByTradeCount{symbols: symbols})
		}
		topNumberOfTrades, _ = b.service.GetTopSymbols(
			SPREAD_METRICS_QUOTE_ASSET, TOP_LIMIT, byTradeCountSort)

		topTradeCountCache.SetDefault(TOP_TRADE_COUNT_KEY, topNumberOfTrades)
	}

	// get spreds
	var spreadTargets []string
	for _, v := range topNumberOfTrades {
		spreadTargets = append(spreadTargets, v.Symbol)
	}
	spreads, _ := b.service.GetSpreads(spreadTargets)

	var delta decimal.Decimal
	newState := make(map[string]*SpreadMetric)
	for _, spread := range spreads {
		if old, found := b.state[spread.Symbol]; found {
			delta = spread.Value.Add(old.spread.Value.Neg())
			b.printSpreadData(spread, delta)
		} else {
			b.printSpreadData(spread, decimal.Zero)
		}
		newState[spread.Symbol] = &SpreadMetric{spread, delta}
	}
	b.state = newState

	// use a cache with auto-expire as a communication channel
	// so prometheus collector will report spread data or none
	// regardless of its scrape interval
	MetricsCache.SetDefault(SPREAD_METRICS_KEY, newState)
}

func (b *background) printSpreadData(spread *Spread, delta decimal.Decimal) {
	// print to logger out
	var deltaSign string
	switch delta.Sign() {
	case 1:
		deltaSign = "+"
	case -1:
		deltaSign = "-"
	case 0:
		deltaSign = "="
	}
	log.Infof("%s: %s (%s%s)", spread.Symbol, spread.Value, deltaSign, delta.Abs())
}
