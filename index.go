package main

import (
	"net/http"
	"text/template"
)

type SymbolsSection struct {
	Title  string
	Values []*SymbolData
}

type NotionalValuesSection struct {
	Title  string
	Values []*TotalNotionalValue
}

type SpreadsSection struct {
	Title  string
	Values []*Spread
}

type PageData struct {
	PageTitle           string
	TopVolumes          SymbolsSection
	TopNumberOfTrades   SymbolsSection
	TotalNotionalValues NotionalValuesSection
	SpreadValues        SpreadsSection
}

func (c *controller) index(w http.ResponseWriter, req *http.Request) {
	client := NewApiClient(apiBaseUrl)
	service := NewMarketDataService(&client)

	marketData, _ := service.GetMarketData(
		&MarketDataQuery{
			VolumeQuoteAsset:     "BTC",
			TradeCountQuoteAsset: "USDT",
		})

	tmpl := template.Must(template.ParseFiles("index.html"))

	data := PageData{
		PageTitle: "Binance Market Data",
		TopVolumes: SymbolsSection{
			Title:  "Top 5 highest volume over the last 24h for quote asset BTC",
			Values: marketData.TopVolumes,
		},
		TopNumberOfTrades: SymbolsSection{
			Title:  "Top 5 highest number of trades over the last 24h for quote asset USDT",
			Values: marketData.TopNumberOfTrades,
		},
		TotalNotionalValues: NotionalValuesSection{
			Title:  "Total notional value of the top 200 bids and asks",
			Values: marketData.TotalNotionalValues,
		},
		SpreadValues: SpreadsSection{
			Title:  "Bid-Ask spread",
			Values: marketData.Spreads,
		},
	}
	tmpl.Execute(w, data)
}
