package types

import (
	"encoding/json"
)

// Base message structures
type WSMessage struct {
	Method       string                 `json:"method,omitempty"`
	Subscription *SubscriptionRequest   `json:"subscription,omitempty"`
	Channel      string                 `json:"channel,omitempty"`
	Data         json.RawMessage        `json:"data,omitempty"`
	ID           *int64                 `json:"id,omitempty"`
	Request      *PostRequest           `json:"request,omitempty"`
}

type SubscriptionRequest struct {
	Type            string  `json:"type"`
	User            string  `json:"user,omitempty"`
	Coin            string  `json:"coin,omitempty"`
	Interval        string  `json:"interval,omitempty"`
	Dex             string  `json:"dex,omitempty"`
	NSigFigs        *int    `json:"nSigFigs,omitempty"`
	Mantissa        *int    `json:"mantissa,omitempty"`
	AggregateByTime *bool   `json:"aggregateByTime,omitempty"`
}

type PostRequest struct {
	Type    string          `json:"type"` // "info" or "action"
	Payload json.RawMessage `json:"payload"`
}

type PostResponse struct {
	ID       int64                  `json:"id"`
	Response PostResponseInner     `json:"response"`
}

type PostResponseInner struct {
	Type    string          `json:"type"` // "info", "action", or "error"
	Payload json.RawMessage `json:"payload"`
}

// Subscription types enum
type SubscriptionType string

const (
	AllMidsType                     SubscriptionType = "allMids"
	L2BookType                      SubscriptionType = "l2Book"
	TradesType                      SubscriptionType = "trades"
	CandleType                      SubscriptionType = "candle"
	BBOType                         SubscriptionType = "bbo"
	NotificationType                SubscriptionType = "notification"
	WebData2Type                    SubscriptionType = "webData2"
	OrderUpdates                SubscriptionType = "orderUpdates"
	UserEvents                  SubscriptionType = "userEvents"
	UserFills                   SubscriptionType = "userFills"
	UserFundings                SubscriptionType = "userFundings"
	UserNonFundingLedgerUpdates SubscriptionType = "userNonFundingLedgerUpdates"
	ActiveAssetCtx              SubscriptionType = "activeAssetCtx"
	ActiveAssetData             SubscriptionType = "activeAssetData"
	UserTwapSliceFills          SubscriptionType = "userTwapSliceFills"
	UserTwapHistory             SubscriptionType = "userTwapHistory"
)

// Response data structures
type AllMids struct {
	Mids map[string]string `json:"mids"`
}

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

type WsBook struct {
	Coin   string      `json:"coin"`
	Levels [2][]WsLevel `json:"levels"`
	Time   int64       `json:"time"`
}

type WsLevel struct {
	Px string `json:"px"`
	Sz string `json:"sz"`
	N  int    `json:"n"`
}

type WsBbo struct {
	Coin string     `json:"coin"`
	Time int64      `json:"time"`
	BBO  [2]*WsLevel `json:"bbo"`
}

type Notification struct {
	Notification string `json:"notification"`
}

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

type WebData2 struct {
	Data map[string]interface{} `json:"data,omitempty"`
}

type WsUserFills struct {
	IsSnapshot *bool    `json:"isSnapshot,omitempty"`
	User       string   `json:"user"`
	Fills      []WsFill `json:"fills"`
}

type WsFill struct {
	Coin          string           `json:"coin"`
	Px            string           `json:"px"`
	Sz            string           `json:"sz"`
	Side          string           `json:"side"`
	Time          int64            `json:"time"`
	StartPosition string           `json:"startPosition"`
	Dir           string           `json:"dir"`
	ClosedPnl     string           `json:"closedPnl"`
	Hash          string           `json:"hash"`
	OID           int64            `json:"oid"`
	Crossed       bool             `json:"crossed"`
	Fee           string           `json:"fee"`
	TID           int64            `json:"tid"`
	Liquidation   *FillLiquidation `json:"liquidation,omitempty"`
	FeeToken      string           `json:"feeToken"`
	BuilderFee    *string          `json:"builderFee,omitempty"`
}

type FillLiquidation struct {
	LiquidatedUser *string `json:"liquidatedUser,omitempty"`
	MarkPx         float64 `json:"markPx"`
	Method         string  `json:"method"` // "market" or "backstop"
}

type WsOrder struct {
	Order           WsBasicOrder `json:"order"`
	Status          string       `json:"status"`
	StatusTimestamp int64        `json:"statusTimestamp"`
}

type WsBasicOrder struct {
	Coin      string  `json:"coin"`
	Side      string  `json:"side"`
	LimitPx   string  `json:"limitPx"`
	Sz        string  `json:"sz"`
	OID       int64   `json:"oid"`
	Timestamp int64   `json:"timestamp"`
	OrigSz    string  `json:"origSz"`
	Cloid     *string `json:"cloid"`
}

type WsUserEvent struct {
	Fills         []WsFill           `json:"fills,omitempty"`
	Funding       *WsUserFunding     `json:"funding,omitempty"`
	Liquidation   *WsLiquidation     `json:"liquidation,omitempty"`
	NonUserCancel []WsNonUserCancel  `json:"nonUserCancel,omitempty"`
}

type WsUserFunding struct {
	Time        int64  `json:"time"`
	Coin        string `json:"coin"`
	Usdc        string `json:"usdc"`
	Szi         string `json:"szi"`
	FundingRate string `json:"fundingRate"`
}

type WsLiquidation struct {
	LID                      int64  `json:"lid"`
	Liquidator               string `json:"liquidator"`
	LiquidatedUser           string `json:"liquidated_user"`
	LiquidatedNtlPos         string `json:"liquidated_ntl_pos"`
	LiquidatedAccountValue   string `json:"liquidated_account_value"`
}

type WsNonUserCancel struct {
	Coin string `json:"coin"`
	OID  int64  `json:"oid"`
}

type WsActiveAssetCtx struct {
	Coin string          `json:"coin"`
	Ctx  PerpsAssetCtx   `json:"ctx"`
}

type WsActiveSpotAssetCtx struct {
	Coin string        `json:"coin"`
	Ctx  SpotAssetCtx  `json:"ctx"`
}

type SharedAssetCtx struct {
	DayNtlVlm  float64  `json:"dayNtlVlm"`
	PrevDayPx  float64  `json:"prevDayPx"`
	MarkPx     float64  `json:"markPx"`
	MidPx      *float64 `json:"midPx,omitempty"`
}

type PerpsAssetCtx struct {
	SharedAssetCtx
	Funding        float64 `json:"funding"`
	OpenInterest   float64 `json:"openInterest"`
	OraclePx       float64 `json:"oraclePx"`
}

type SpotAssetCtx struct {
	SharedAssetCtx
	CirculatingSupply float64 `json:"circulatingSupply"`
}

type WsActiveAssetData struct {
	User             string     `json:"user"`
	Coin             string     `json:"coin"`
	Leverage         Leverage   `json:"leverage"`
	MaxTradeSzs      [2]float64 `json:"maxTradeSzs"`
	AvailableToTrade [2]float64 `json:"availableToTrade"`
}

type Leverage struct {
	Type  string `json:"type"`
	Value int    `json:"value"`
}

// Additional types for ledger updates
type WsUserNonFundingLedgerUpdates struct {
	IsSnapshot *bool                            `json:"isSnapshot,omitempty"`
	User       string                           `json:"user"`
	Updates    []WsUserNonFundingLedgerUpdate   `json:"updates"`
}

type WsUserNonFundingLedgerUpdate struct {
	Time  int64          `json:"time"`
	Hash  string         `json:"hash"`
	Delta WsLedgerUpdate `json:"delta"`
}

type WsLedgerUpdate struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// TWAP related types
type WsUserTwapSliceFills struct {
	IsSnapshot     *bool              `json:"isSnapshot,omitempty"`
	User           string             `json:"user"`
	TwapSliceFills []WsTwapSliceFill  `json:"twapSliceFills"`
}

type WsTwapSliceFill struct {
	Fill   WsFill `json:"fill"`
	TwapID int64  `json:"twapId"`
}

type WsUserTwapHistory struct {
	IsSnapshot *bool            `json:"isSnapshot,omitempty"`
	User       string           `json:"user"`
	History    []WsTwapHistory  `json:"history"`
}

type WsTwapHistory struct {
	State  TwapState   `json:"state"`
	Status TwapStatus  `json:"status"`
	Time   int64       `json:"time"`
}

type TwapState struct {
	Coin        string  `json:"coin"`
	User        string  `json:"user"`
	Side        string  `json:"side"`
	Sz          float64 `json:"sz"`
	ExecutedSz  float64 `json:"executedSz"`
	ExecutedNtl float64 `json:"executedNtl"`
	Minutes     int     `json:"minutes"`
	ReduceOnly  bool    `json:"reduceOnly"`
	Randomize   bool    `json:"randomize"`
	Timestamp   int64   `json:"timestamp"`
}

type TwapStatus struct {
	Status      string `json:"status"` // "activated" | "terminated" | "finished" | "error"
	Description string `json:"description"`
}

type WsUserFundings struct {
	IsSnapshot *bool            `json:"isSnapshot,omitempty"`
	User       string           `json:"user"`
	Fundings   []WsUserFunding  `json:"fundings"`
} 