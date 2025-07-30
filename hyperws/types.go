package main

import (
	"encoding/json"
)

// Message WebSocket de base
type WSMessage struct {
	Method       string                 `json:"method,omitempty"`
	Subscription *SubscriptionRequest   `json:"subscription,omitempty"`
	Channel      string                 `json:"channel,omitempty"`
	Data         json.RawMessage        `json:"data,omitempty"`
	ID           *int64                 `json:"id,omitempty"`
}

// Requête de souscription
type SubscriptionRequest struct {
	Type     string `json:"type"`
	User     string `json:"user,omitempty"`
	Coin     string `json:"coin,omitempty"`
	Interval string `json:"interval,omitempty"`
}

// Types de souscription supportés
const (
	AllMidsType         = "allMids"
	L2BookType          = "l2Book"
	TradesType          = "trades"
	CandleType          = "candle"
	BBOType             = "bbo"
	NotificationType    = "notification"
	WebData2Type        = "webData2"
	OrderUpdatesType    = "orderUpdates"
	UserEventsType      = "userEvents"
	UserFillsType       = "userFills"
	UserFundingsType    = "userFundings"
	UserLedgerType      = "userNonFundingLedgerUpdates"
	ActiveAssetCtxType  = "activeAssetCtx"
	ActiveAssetDataType = "activeAssetData"
	UserTwapFillsType   = "userTwapSliceFills"
	UserTwapHistoryType = "userTwapHistory"
)

// Données des prix moyens
type AllMids struct {
	Mids map[string]string `json:"mids"`
}

// Données de trade
type WsTrade struct {
	Coin  string    `json:"coin"`
	Side  string    `json:"side"`
	Px    string    `json:"px"`
	Sz    string    `json:"sz"`
	Hash  string    `json:"hash"`
	Time  int64     `json:"time"`
	TID   int64     `json:"tid"`
	Users [2]string `json:"users"`
}

// Niveau de book (bid/ask)
type WsLevel struct {
	Px string `json:"px"`
	Sz string `json:"sz"`
	N  int    `json:"n"`
}

// Données du book L2
type WsBook struct {
	Coin   string        `json:"coin"`
	Levels [2][]WsLevel  `json:"levels"`
	Time   int64         `json:"time"`
}

// Best Bid/Offer
type WsBBO struct {
	Coin string      `json:"coin"`
	Time int64       `json:"time"`
	BBO  [2]*WsLevel `json:"bbo"`
}

// Données de chandelier
type Candle struct {
	T int64   `json:"t"` // open millis
	T2 int64  `json:"T"` // close millis
	S string  `json:"s"` // coin
	I string  `json:"i"` // interval
	O float64 `json:"o"` // open price
	C float64 `json:"c"` // close price
	H float64 `json:"h"` // high price
	L float64 `json:"l"` // low price
	V float64 `json:"v"` // volume
	N int     `json:"n"` // number of trades
}

// Asset metadata pour résolution dynamique
type AssetInfo struct {
	Name     string `json:"name"`
	AssetID  int    `json:"assetId"`
	Universe int    `json:"universe"`
} 