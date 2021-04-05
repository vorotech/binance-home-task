package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
)

var (
	once     sync.Once
	instance *client
)

const EXCHANGE_INFO_KEY = "exchangeInfo"

type ApiClient interface {
	GetExchangeInfo() (*ExchangeInfoResponse, error)
	GetTickerChangeStatistics(symbol string) ([]*TickerChangeStatics, error)
	GetOrderBook(symbol string, limit int) (*OrderBook, error)
}

type client struct {
	apiBaseUrl  string
	infoCache   *cache.Cache
	tickerCache *cache.Cache
}

func NewApiClient(baseUrl string) ApiClient {

	once.Do(func() {
		instance = &client{
			apiBaseUrl:  baseUrl,
			infoCache:   cache.New(time.Duration(10)*time.Minute, time.Duration(10)*time.Minute),
			tickerCache: cache.New(time.Duration(1)*time.Second, time.Duration(1)*time.Second),
		}
	})

	return instance
}

func (c *client) GetExchangeInfo() (*ExchangeInfoResponse, error) {
	var info *ExchangeInfoResponse
	if x, found := c.infoCache.Get(EXCHANGE_INFO_KEY); found {
		info = x.(*ExchangeInfoResponse)
		log.Debug("Used cache to get exchange info")
		return info, nil
	}

	err := c.restRequest(http.MethodGet, "/api/v3/exchangeInfo", nil, &info, nil)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	c.infoCache.SetDefault(EXCHANGE_INFO_KEY, info)

	return info, nil
}

func (c *client) GetTickerChangeStatistics(symbol string) ([]*TickerChangeStatics, error) {
	var stats []*TickerChangeStatics
	if x, found := c.tickerCache.Get(symbol); found {
		stats = x.([]*TickerChangeStatics)
		log.Debug("Used cache to get ticker change statistics")
		return stats, nil
	}

	if symbol != "" {
		v := url.Values{}
		v.Set("symbol", symbol)
		var item TickerChangeStatics
		err := c.restRequest(http.MethodGet, "/api/v3/ticker/24hr", nil, &item, v)
		if err != nil {
			log.Error(err.Error())
			return nil, err
		}
		stats = []*TickerChangeStatics{&item}
	} else {
		err := c.restRequest(http.MethodGet, "/api/v3/ticker/24hr", nil, &stats, nil)
		if err != nil {
			log.Error(err.Error())
			return nil, err
		}
	}

	c.tickerCache.SetDefault(symbol, stats)

	return stats, nil
}

func (c *client) GetOrderBook(symbol string, limit int) (*OrderBook, error) {
	v := url.Values{}
	v.Set("limit", strconv.Itoa(limit))
	v.Set("symbol", symbol)

	var orderBook OrderBook
	err := c.restRequest(http.MethodGet, "/api/v3/depth", nil, &orderBook, v)
	if err != nil {
		return nil, err
	}

	return &orderBook, nil
}

func (c *client) restRequest(verb string, path string, payload interface{},
	response interface{}, params url.Values) error {

	url := c.apiBaseUrl + path
	if len(params) != 0 {
		url = updateUri(url, params)
	}

	var body io.Reader
	if payload != nil {
		jsonStr, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewBuffer(jsonStr)
	}

	req, err := http.NewRequest(verb, url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	log.Debugf("Starting request %s %s", verb, url)
	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return err
	}

	if weight := res.Header.Get("x-mbx-used-weight"); weight != "" {
		log.WithField("weight-used", weight).Debugf("Completed request %s %s", verb, url)
	} else {
		log.Debugf("Completed request %s %s", verb, url)
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		var apierr ApiError
		if err = json.Unmarshal(data, &apierr); err != nil {
			return err
		}
		return &apierr
	}

	return json.Unmarshal(data, &response)
}

func updateUri(uri string, params url.Values) string {
	if strings.Contains(uri, "?") {
		uri += "&"
	} else {
		uri += "?"
	}
	return uri + params.Encode()
}
