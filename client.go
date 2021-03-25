package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
)

const (
	EXCHANGE_INFO_KEY = "exchangeInfo"
	TICKER_KEY        = "ticker"
)

var clientCache = cache.New(time.Duration(1)*time.Second, time.Duration(1)*time.Second)

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
	var info *ExchangeInfoResponse
	if x, found := clientCache.Get(EXCHANGE_INFO_KEY); found {
		info = x.(*ExchangeInfoResponse)
		log.Debug("Used cache to get exchange info")
		return info, nil
	}

	u := apiBaseUrl + "/api/v3/exchangeInfo"
	log.Debugf("GET %s", u)
	response, err := http.Get(u)

	if err != nil {
		log.Fatal(err.Error())
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(responseData, &info)
	if err != nil {
		log.Fatal(err)
	}

	clientCache.Set(EXCHANGE_INFO_KEY, info, time.Duration(10)*time.Minute)

	weight := response.Header.Get("x-mbx-used-weight")
	log.WithField("weight-used", weight).Debug("Completed request to get exchange info")
	return info, nil
}

func (c *client) GetTickerChangeStatistics(symbol string) ([]*TickerChangeStatics, error) {
	var stats []*TickerChangeStatics
	if x, found := clientCache.Get(TICKER_KEY + symbol); found {
		stats = x.([]*TickerChangeStatics)
		log.Debug("Used cache to get ticker change statistics")
		return stats, nil
	}

	var u string
	var isArray bool
	if symbol != "" {
		log.WithField("symbol", symbol).Debugf("Get ticker change statistics for %s", symbol)
		u = apiBaseUrl + "/api/v3/ticker/24hr?symbol=" + symbol
		isArray = false
	} else {
		log.Debug("Get ticker change statistics for all symbols")
		u = apiBaseUrl + "/api/v3/ticker/24hr"
		isArray = true
	}

	log.Debugf("GET %s", u)
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

	if isArray {
		err = json.Unmarshal(responseData, &stats)
	} else {
		var item TickerChangeStatics
		err = json.Unmarshal(responseData, &item)
		stats = []*TickerChangeStatics{&item}
	}
	if err != nil {
		log.Error(err)
		return nil, err
	}

	clientCache.Set(TICKER_KEY+symbol, stats, cache.DefaultExpiration)

	weight := response.Header.Get("x-mbx-used-weight")
	log.WithFields(log.Fields{
		"count":       len(stats),
		"weight-used": weight,
	}).Debug("Completed request to get ticker change statistics")
	return stats, nil
}

func (c *client) GetOrderBook(symbol string, limit int) (*OrderBook, error) {
	v := url.Values{}
	v.Set("limit", strconv.Itoa(limit))
	v.Set("symbol", symbol)
	u := apiBaseUrl + "/api/v3/depth?" + v.Encode()

	log.Debugf("GET %s", u)
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

	weight := response.Header.Get("x-mbx-used-weight")
	log.WithFields(log.Fields{
		"symbol":      symbol,
		"weight-used": weight,
	}).Debug("Completed request to get order book")
	return &orderBook, nil
}
