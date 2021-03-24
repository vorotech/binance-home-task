package main

import (
	"sort"
	"strconv"

	log "github.com/sirupsen/logrus"
)

const (
	NO_VALUE         string = ""
	ITEMS_COUNT      int    = 5
	ORDER_BOOK_COUNT        = 200
	ORDER_BOOK_LIMIT        = 500
)

type MarketDataQuery struct {
	VolumeQuoteAsset         string
	NumberOfTradesQuoteAsset string
}

type MarketData struct {
	TopVolume           []*TickerChangeStatics
	TopNumberOfTrades   []*TickerChangeStatics
	TotalNotionalValues []*TotalNotionalValue
}

type TotalNotionalValue struct {
	Symbol    string
	AsksTotal float64
	BidsTotal float64
}

type stats []*TickerChangeStatics

func (s stats) Len() int      { return len(s) }
func (s stats) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// ByVolume implements sort.Interface
type ByVolume struct{ stats }

func (s ByVolume) Less(i, j int) bool { return s.stats[i].Volume > s.stats[j].Volume }

// ByVolume implements sort.Interface
type ByNumberOfTrades struct{ stats }

func (s ByNumberOfTrades) Less(i, j int) bool { return s.stats[i].Tradecount > s.stats[j].Tradecount }

type MarketDataService interface {
	GetMarketData(query *MarketDataQuery) (*MarketData, error)
}

type service struct {
	client ApiClient
}

func NewMarketDataService(c *ApiClient) MarketDataService {
	return &service{
		client: *c,
	}
}

func (s *service) GetMarketData(q *MarketDataQuery) (*MarketData, error) {
	info, err := s.client.GetExchangeInfo()
	if err != nil {
		log.Error("Error occurred while getting exchange info")
		return nil, err
	}

	log.Infof("Get %d symbols from exchange info", len(info.Symbols))
	metadata := make(map[string]Symbol, len(info.Symbols))
	for i := range info.Symbols {
		s := info.Symbols[i]
		metadata[s.Symbol] = s
	}

	tickerStats, err := s.client.GetTickerChangeStatistics(NO_VALUE)
	if err != nil {
		log.Error("Error occurred while getting ticker change statistics")
		return nil, err
	}

	var volumes []*TickerChangeStatics
	var numberOfTrades []*TickerChangeStatics
	for _, t := range tickerStats {
		s := metadata[t.Symbol]
		if s.Quoteasset == q.VolumeQuoteAsset {
			volumes = append(volumes, t)
		}
		if s.Quoteasset == q.NumberOfTradesQuoteAsset {
			numberOfTrades = append(numberOfTrades, t)
		}
	}
	log.WithField("quoteAsset", q.VolumeQuoteAsset).Infof("Found %d symbols to sort by volume", len(volumes))
	log.WithField("quoteAsset", q.NumberOfTradesQuoteAsset).Infof("Found %d symbols to sort by number of trades", len(numberOfTrades))

	sort.Sort(ByVolume{volumes})
	sort.Sort(ByNumberOfTrades{numberOfTrades})

	topVolumes := volumes[:ITEMS_COUNT]

	var totalNotionalValues = make([]*TotalNotionalValue, ITEMS_COUNT)
	var sem = make(chan int, ITEMS_COUNT) // semaphore pattern
	for i, v := range topVolumes {
		v1 := v
		i1 := i
		go func() {
			value, err := s.getTotalNotionalValue(v1.Symbol)
			if err == nil {
				totalNotionalValues[i1] = value
			}
			sem <- 1
		}()
	}

	log.Info("Wait for goroutines to finish")
	for i := 0; i < ITEMS_COUNT; i++ {
		<-sem
	}
	log.Info("All goroutines finished")

	return &MarketData{
		TopVolume:           topVolumes,
		TopNumberOfTrades:   numberOfTrades[:ITEMS_COUNT],
		TotalNotionalValues: totalNotionalValues,
	}, nil
}

func (s *service) getTotalNotionalValue(symbol string) (*TotalNotionalValue, error) {
	book, err := s.client.GetOrderBook(symbol, ORDER_BOOK_LIMIT)
	if err != nil {
		log.WithField("symbol", symbol).Errorf("Error occurred while getting order book for %s", symbol)
		return nil, err
	}

	var asksTotal, bidsTotal, price, qty float64
	if len(book.Asks) > ORDER_BOOK_COUNT {
		book.Asks = book.Asks[:ORDER_BOOK_COUNT]
	}
	for _, v := range book.Asks {
		price, _ = strconv.ParseFloat(v[0], 64)
		qty, _ = strconv.ParseFloat(v[1], 64)
		asksTotal += price * qty
	}

	if len(book.Bids) > ORDER_BOOK_COUNT {
		book.Bids = book.Asks[:ORDER_BOOK_COUNT]
	}
	for _, v := range book.Bids {
		price, _ = strconv.ParseFloat(v[0], 64)
		qty, _ = strconv.ParseFloat(v[1], 64)
		bidsTotal += price * qty
	}

	return &TotalNotionalValue{
		Symbol:    symbol,
		AsksTotal: asksTotal,
		BidsTotal: bidsTotal,
	}, nil
}
