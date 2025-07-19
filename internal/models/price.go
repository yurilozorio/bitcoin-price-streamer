package models

import "time"

// PriceUpdate represents a Bitcoin price update
type PriceUpdate struct {
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
	Symbol    string    `json:"symbol"`
	Name      string    `json:"name"`
}

// CoinDeskResponse represents the response from the new CoinDesk API
type CoinDeskResponse struct {
	Data struct {
		Stats struct {
			Page        int `json:"PAGE"`
			PageSize    int `json:"PAGE_SIZE"`
			TotalAssets int `json:"TOTAL_ASSETS"`
		} `json:"STATS"`
		List []AssetData `json:"LIST"`
	} `json:"Data"`
}

// AssetData represents individual asset data from the API
type AssetData struct {
	ID                                  int     `json:"ID"`
	Symbol                              string  `json:"SYMBOL"`
	Name                                string  `json:"NAME"`
	PriceUSD                            float64 `json:"PRICE_USD"`
	PriceUSDSource                      string  `json:"PRICE_USD_SOURCE"`
	PriceUSDLastUpdateTS                int64   `json:"PRICE_USD_LAST_UPDATE_TS"`
	PriceConversionValue                float64 `json:"PRICE_CONVERSION_VALUE"`
	PriceConversionSource               string  `json:"PRICE_CONVERSION_SOURCE"`
	PriceConversionLastUpdateTS         int64   `json:"PRICE_CONVERSION_LAST_UPDATE_TS"`
	SpotMoving24HourChangeUSD           float64 `json:"SPOT_MOVING_24_HOUR_CHANGE_USD"`
	SpotMoving24HourChangePercentageUSD float64 `json:"SPOT_MOVING_24_HOUR_CHANGE_PERCENTAGE_USD"`
	CirculatingMktCapUSD                float64 `json:"CIRCULATING_MKT_CAP_USD"`
	TotalMktCapUSD                      float64 `json:"TOTAL_MKT_CAP_USD"`
	SpotMoving24HourQuoteVolumeUSD      float64 `json:"SPOT_MOVING_24_HOUR_QUOTE_VOLUME_USD"`
	UpdatedOn                           int64   `json:"UPDATED_ON"`
}
