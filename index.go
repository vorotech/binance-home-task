package main

import (
	"net/http"
	"text/template"
)

type StatsSection struct {
	Title string
	Stats []*TickerChangeStatics
}

type NotionalValuesSection struct {
	Title  string
	Values []*TotalNotionalValue
}

type PageData struct {
	PageTitle           string
	TopVolumes          StatsSection
	TopNumberOfTrades   StatsSection
	TotalNotionalValues NotionalValuesSection
}

func (c *controller) index(w http.ResponseWriter, req *http.Request) {
	client := NewApiClient()
	service := NewMarketDataService(&client)

	marketData, _ := service.GetMarketData(
		&MarketDataQuery{
			VolumeQuoteAsset:         "BTC",
			NumberOfTradesQuoteAsset: "USDT",
		})

	tmpl := template.Must(template.ParseFiles("index.html"))

	data := PageData{
		PageTitle: "Binance Market Data",
		TopVolumes: StatsSection{
			Title: "Top 5 highest volume over the last 24h for quote asset BTC",
			Stats: marketData.TopVolume,
		},
		TopNumberOfTrades: StatsSection{
			Title: "Top 5 highest number of trades over the last 24h for quote asset USDT",
			Stats: marketData.TopNumberOfTrades,
		},
		TotalNotionalValues: NotionalValuesSection{
			Title:  "Total notional value of the top 200 bids and asks",
			Values: marketData.TotalNotionalValues,
		},
	}
	tmpl.Execute(w, data)
}
