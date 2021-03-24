package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	log "github.com/sirupsen/logrus"
)

type ApiClient interface {
	GetExchangeInfo() (*ExchangeInfoResponse, error)
	GetTickerChangeStatistics(symbol string) ([]*TickerChangeStatics, error)
	GetOrderBook(symbol string, limit int) (*OrderBook, error)
}

type client struct{}

func NewApiClient() ApiClient {
	return &client{}
}

func (c *client) GetExchangeInfo() (*ExchangeInfoResponse, error) {
	u := apiBaseUrl + "/api/v3/exchangeInfo"
	log.Infof("GET %s", u)
	response, err := http.Get(u)

	if err != nil {
		log.Fatal(err.Error())
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	var exchangeInfo ExchangeInfoResponse
	err = json.Unmarshal(responseData, &exchangeInfo)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Completed request to get exchange info")
	return &exchangeInfo, nil
}

func (c *client) GetTickerChangeStatistics(symbol string) ([]*TickerChangeStatics, error) {
	var u string
	var isArray bool
	if symbol != "" {
		log.WithField("symbol", symbol).Infof("Get ticker change statistics for %s", symbol)
		u = apiBaseUrl + "/api/v3/ticker/24hr?symbol=" + symbol
		isArray = false
	} else {
		log.Info("Get ticker change statistics for all symbols")
		u = apiBaseUrl + "/api/v3/ticker/24hr"
		isArray = true
	}

	log.Infof("GET %s", u)
	response, err := http.Get(u)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	var tickerChangeStatics []TickerChangeStatics
	if isArray {
		err = json.Unmarshal(responseData, &tickerChangeStatics)
	} else {
		var item TickerChangeStatics
		err = json.Unmarshal(responseData, &item)
		tickerChangeStatics = []TickerChangeStatics{item}
	}
	if err != nil {
		log.Error(err)
		return nil, err
	}

	log.WithField("count", len(tickerChangeStatics)).Info("Completed request to get ticker change statistics")

	var result = []*TickerChangeStatics{}
	for i := range tickerChangeStatics {
		result = append(result, &tickerChangeStatics[i])
	}
	return result, nil
}

func (c *client) GetOrderBook(symbol string, limit int) (*OrderBook, error) {
	v := url.Values{}
	v.Set("limit", strconv.Itoa(limit))
	v.Set("symbol", symbol)
	u := apiBaseUrl + "/api/v3/depth?" + v.Encode()

	log.Infof("GET %s", u)
	response, err := http.Get(u)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		var apierr ApiError
		if err = json.Unmarshal(responseData, &apierr); err != nil {
			log.Error(err)
			return nil, err
		}
		log.WithField("code", apierr.Code).Error(apierr.Message)
		return nil, &apierr
	}

	var orderBook OrderBook
	err = json.Unmarshal(responseData, &orderBook)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	log.WithField("symbol", symbol).Infof("Completed request to get order book for %s", symbol)
	return &orderBook, nil
}
