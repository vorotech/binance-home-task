package main

import (
	"errors"
	"sort"

	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

const (
	NO_VALUE  string = ""
	TOP_LIMIT int    = 5
)

type MarketDataQuery struct {
	VolumeQuoteAsset     string
	TradeCountQuoteAsset string
}

type MarketData struct {
	TopVolumes          []*SymbolData
	TopNumberOfTrades   []*SymbolData
	TotalNotionalValues []*TotalNotionalValue
	Spreads             []*Spread
}

type SymbolData struct {
	Symbol     string
	Volume     decimal.Decimal
	TradeCount int
}

type TotalNotionalValue struct {
	Symbol    string
	AsksTotal decimal.Decimal
	BidsTotal decimal.Decimal
}

type Spread struct {
	Symbol     string
	HighestBid decimal.Decimal
	LowestAsk  decimal.Decimal
	Value      decimal.Decimal
}

type MarketDataService interface {
	GetMarketData(query *MarketDataQuery) (*MarketData, error)
	GetTopSymbols(quoteAsset string, limit int, sort func(symbols []*SymbolData)) ([]*SymbolData, error)
	GetTotalNotionalValues(symbols []string) ([]*TotalNotionalValue, error)
	GetSpreads(symbols []string) ([]*Spread, error)
}

type service struct {
	client   ApiClient
	metadata map[string]Symbol
}

func NewMarketDataService(c *ApiClient) MarketDataService {
	info, err := (*c).GetExchangeInfo()
	if err != nil {
		log.Fatal("Error occurred while getting exchange info")
	}

	metadata := make(map[string]Symbol, len(info.Symbols))
	for i := range info.Symbols {
		s := info.Symbols[i]
		metadata[s.Symbol] = s
	}

	return &service{
		client:   *c,
		metadata: metadata,
	}
}

func (s *service) GetMarketData(q *MarketDataQuery) (*MarketData, error) {

	// get top volumes
	byVolumeSort := func(symbols []*SymbolData) {
		sort.Sort(ByVolume{symbols: symbols})
	}
	topVolumes, _ := s.GetTopSymbols(
		q.VolumeQuoteAsset, TOP_LIMIT, byVolumeSort)

	// get top number of trades
	byTradeCountSort := func(symbols []*SymbolData) {
		sort.Sort(ByTradeCount{symbols: symbols})
	}
	topNumberOfTrades, _ := s.GetTopSymbols(
		q.TradeCountQuoteAsset, TOP_LIMIT, byTradeCountSort)

	// get total notional values
	var tnvTargets []string
	for _, v := range topVolumes {
		tnvTargets = append(tnvTargets, v.Symbol)
	}
	totalNotionalValues, _ := s.GetTotalNotionalValues(tnvTargets)

	// get spreds
	var spreadTargets []string
	for _, v := range topNumberOfTrades {
		spreadTargets = append(spreadTargets, v.Symbol)
	}
	spreads, _ := s.GetSpreads(spreadTargets)

	return &MarketData{
		TopVolumes:          topVolumes,
		TopNumberOfTrades:   topNumberOfTrades,
		TotalNotionalValues: totalNotionalValues,
		Spreads:             spreads,
	}, nil
}

func (s *service) GetTopSymbols(
	quoteAsset string, limit int, sort func(symbols []*SymbolData),
) ([]*SymbolData, error) {
	stats, err := s.client.GetTickerChangeStatistics(NO_VALUE)
	if err != nil {
		log.Error("Error occurred while getting ticker change statistics")
		return nil, err
	}

	var symbols []*SymbolData
	for _, t := range stats {
		s := s.metadata[t.Symbol]
		if s.Quoteasset == quoteAsset {
			vol, _ := decimal.NewFromString(t.Volume)
			symbols = append(symbols, &SymbolData{
				Symbol:     t.Symbol,
				Volume:     vol,
				TradeCount: t.Tradecount,
			})
		}
	}

	log.WithField("quoteAsset", quoteAsset).Debugf(
		"Found %d symbols to sort", len(symbols))

	sort(symbols)

	if len(symbols) > limit {
		symbols = symbols[:limit]
	}

	return symbols, nil
}

func (s *service) GetTotalNotionalValues(symbols []string) ([]*TotalNotionalValue, error) {
	var aerr error
	var tnvs []*TotalNotionalValue
	c := make(chan *TotalNotionalValue)
	for _, symbol := range symbols {
		s1 := symbol
		go func() {
			value, err := s.getTotalNotionalValue(s1)
			if err != nil {
				aerr = err
			}
			c <- value
		}()
	}

	for i := 0; i < len(symbols); i++ {
		tnvs = append(tnvs, <-c)
	}

	if aerr != nil {
		log.Error("Erorr occurred while getting total notional values")
		return nil, aerr
	}

	return tnvs, nil
}

func (s *service) getTotalNotionalValue(symbol string) (*TotalNotionalValue, error) {
	limit := 500
	count := 200

	book, err := s.client.GetOrderBook(symbol, limit)
	if err != nil {
		log.WithField("symbol", symbol).Errorf(
			"Error occurred while getting order book for %s", symbol)
		return nil, err
	}

	var asksTotal, bidsTotal, price, qty decimal.Decimal
	if len(book.Asks) > count {
		book.Asks = book.Asks[:count]
	}
	for _, v := range book.Asks {
		price, _ = decimal.NewFromString(v[0])
		qty, _ = decimal.NewFromString(v[1])
		asksTotal = asksTotal.Add(price.Mul(qty))
	}

	if len(book.Bids) > count {
		book.Bids = book.Asks[:count]
	}
	for _, v := range book.Bids {
		price, _ = decimal.NewFromString(v[0])
		qty, _ = decimal.NewFromString(v[1])
		bidsTotal = bidsTotal.Add(price.Mul(qty))
	}

	return &TotalNotionalValue{
		Symbol:    symbol,
		AsksTotal: asksTotal,
		BidsTotal: bidsTotal,
	}, nil
}

func (s *service) GetSpreads(symbols []string) ([]*Spread, error) {
	var aerr error
	var spreads []*Spread
	c := make(chan *Spread)
	for _, symbol := range symbols {
		s1 := symbol
		go func() {
			value, err := s.getSpread(s1)
			if err != nil {
				aerr = err
			}
			c <- value
		}()
	}

	for i := 0; i < len(symbols); i++ {
		spreads = append(spreads, <-c)
	}
	close(c)

	if aerr != nil {
		log.Error("Erorr occurred while getting spreads")
		return nil, aerr
	}

	return spreads, nil
}

func (s *service) getSpread(symbol string) (*Spread, error) {
	book, err := s.client.GetOrderBook(symbol, 5)
	if err != nil {
		log.WithField("symbol", symbol).Errorf(
			"Error occurred while getting order book for %s", symbol)
		return nil, err
	}

	// not sure if this edge case is possible and how to calc spread
	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		return nil, errors.New("empty bids or asks in order book")
	}

	var hbid, lask, spread decimal.Decimal
	hbid, _ = decimal.NewFromString(book.Bids[0][0])
	lask, _ = decimal.NewFromString(book.Asks[0][0])
	spread = lask.Add(hbid.Neg())

	return &Spread{
		Symbol:     symbol,
		HighestBid: hbid,
		LowestAsk:  lask,
		Value:      spread,
	}, nil
}
