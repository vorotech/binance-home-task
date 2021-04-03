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
	"time"

	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
)

const (
	EXCHANGE_INFO_KEY = "exchangeInfo"
)

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
	return &client{
		apiBaseUrl:  baseUrl,
		infoCache:   cache.New(time.Duration(10)*time.Minute, time.Duration(10)*time.Minute),
		tickerCache: cache.New(time.Duration(1)*time.Second, time.Duration(1)*time.Second),
	}
}

func (c *client) GetExchangeInfo() (*ExchangeInfoResponse, error) {
	var info *ExchangeInfoResponse
	if x, found := c.infoCache.Get(EXCHANGE_INFO_KEY); found {
		info = x.(*ExchangeInfoResponse)
		log.Debug("Used cache to get exchange info")
		return info, nil
	}

	err := c.restRequest(http.MethodGet, "/api/v3/exchangeInfo", &info)
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
		err := c.restRequest(http.MethodGet, "/api/v3/ticker/24hr", &item, setParams(v))
		if err != nil {
			log.Error(err.Error())
			return nil, err
		}
		stats = []*TickerChangeStatics{&item}
	} else {
		err := c.restRequest(http.MethodGet, "/api/v3/ticker/24hr", &stats)
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
	err := c.restRequest(http.MethodGet, "/api/v3/depth", &orderBook, setParams(v))
	if err != nil {
		return nil, err
	}

	return &orderBook, nil
}

type ApiOptions struct {
	payload interface{}
	params  url.Values
}

type ApiOptionsParams func(options *ApiOptions) error

func setParams(params url.Values) func(*ApiOptions) error {
	return func(opts *ApiOptions) error {
		opts.params = params
		return nil
	}
}

// func setPayload(payload interface{}) func(*ApiOptions) error {
// 	return func(opts *ApiOptions) error {
// 		opts.payload = payload
// 		return nil
// 	}
// }

func (c *client) restRequest(verb string, path string, response interface{}, options ...ApiOptionsParams) error {
	opts, err := getOptions(options)
	if err != nil {
		return err
	}

	url := c.apiBaseUrl + path
	if len(opts.params) != 0 {
		url = updateUri(url, opts)
	}

	var body io.Reader
	if opts.payload != nil {
		jsonStr, err := json.Marshal(opts.payload)
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

func getOptions(options []ApiOptionsParams) (*ApiOptions, error) {
	opts := &ApiOptions{}
	for _, opt := range options {
		err := opt(opts)
		if err != nil {
			return opts, err
		}
	}
	return opts, nil
}

func updateUri(uri string, opts *ApiOptions) string {
	if strings.Contains(uri, "?") {
		uri += "&"
	} else {
		uri += "?"
	}
	return uri + opts.params.Encode()
}
