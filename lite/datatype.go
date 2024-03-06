package lite

import (
	"fmt"
	"log"
	"time"

	"github.com/pseudocodes/go2ctp/thost"
)

const (
	DefaultLayout = "20060102 15:04:05"
)

type Order struct {
	InstrumentID    string
	Exchange        string
	UserID          string
	ExchangeOrderID string // OrderSysID
	VolumeOrigin    int
	VolumeLeft      int
	VolumeTraded    int
	LimitPrice      float64

	Direction      thost.TThostFtdcDirectionType
	Offset         thost.TThostFtdcOffsetFlagType
	PriceType      thost.TThostFtdcOrderPriceTypeType
	TimeCond       thost.TThostFtdcTimeConditionType
	VolumeCond     thost.TThostFtdcVolumeConditionType
	InsertDateTime int64

	FrozenMargin float64
	SubmitStatus thost.TThostFtdcOrderSubmitStatusType
	Status       thost.TThostFtdcOrderStatusType
	LastMsg      string

	FrontID   int32
	SessionID int32
	OrderRef  string
	OrderID   string
}

func OrderFrom(order *Order, f *thost.CThostFtdcOrderField) *Order {
	order.InstrumentID = f.InstrumentID.String()
	order.Exchange = f.ExchangeID.String()
	order.UserID = f.UserID.String()
	order.ExchangeOrderID = f.OrderSysID.String()
	order.VolumeOrigin = int(f.VolumeTotalOriginal)
	order.VolumeLeft = int(f.VolumeTotal)
	order.VolumeTraded = int(f.VolumeTraded)
	order.Direction = f.Direction
	order.Offset = thost.TThostFtdcOffsetFlagType(f.CombOffsetFlag[0])
	order.PriceType = f.OrderPriceType
	order.TimeCond = f.TimeCondition
	order.VolumeCond = f.VolumeCondition
	order.LimitPrice = float64(f.LimitPrice)
	order.Status = f.OrderStatus
	order.SubmitStatus = f.OrderSubmitStatus
	order.LastMsg = f.StatusMsg.GBString()
	order.FrontID = int32(f.FrontID)
	order.SessionID = int32(f.SessionID)
	order.OrderRef = f.OrderRef.String()
	order.OrderID = GetOrderKey(order.FrontID, order.SessionID, order.OrderRef)

	tt, err := time.ParseInLocation(DefaultLayout, f.InsertDate.String()+" "+f.InsertTime.String(), time.Local)
	if err != nil {
		log.Printf("parse time in local error")
	}
	order.InsertDateTime = tt.UnixNano()
	return order
}

func OrderFromInput(f *thost.CThostFtdcInputOrderField) *Order {
	order := &Order{
		InstrumentID:   f.InstrumentID.String(),
		Exchange:       f.ExchangeID.String(),
		UserID:         f.UserID.String(),
		VolumeOrigin:   int(f.VolumeTotalOriginal),
		Direction:      f.Direction,
		Offset:         thost.TThostFtdcOffsetFlagType(f.CombOffsetFlag[0]),
		PriceType:      f.OrderPriceType,
		TimeCond:       f.TimeCondition,
		VolumeCond:     f.VolumeCondition,
		InsertDateTime: time.Now().UnixNano(),
		LimitPrice:     float64(f.LimitPrice),
		Status:         thost.THOST_FTDC_OST_Unknown,
		SubmitStatus:   thost.THOST_FTDC_OSS_InsertSubmitted,
		// OrderRef:       f.OrderRef.String(),
	}

	return order

}

func GetOrderKey(frontID int32, session int32, orderRef string) string {
	return fmt.Sprintf("%s.%8x.%d", orderRef, session, frontID)
}

type Position struct {
	Instrument    string
	Exchange      string
	VolumeLong    int // 多总仓
	VolumeLongTd  int // 多今仓
	VolumeLongHis int // 多昨仓

	VolumeLongFrozen    int // 多头冻结手数
	VolumeLongFrozenTd  int // 多头今仓冻结手数
	VolumeLongFrozenHis int // 多头冻结手数

	VolumeShort    int // 空总仓
	VolumeShortTd  int // 空今仓
	VolumeShortHis int // 空昨仓

	VolumeShortFrozen    int // 空头冻结手数
	VolumeShortFrozenTd  int // 空头今仓冻结手数
	VolumeShortFrozenHis int // 空头昨仓冻结手数

	VolumeLongYd  int
	VolumeShortYd int

	// 成本, 现价与盈亏
	OpenPriceLong  float64 `json:"open_price_long" csv:"open_price_long"`   // 多头开仓均价
	OpenPriceShort float64 `json:"open_price_short" csv:"open_price_short"` // 空头开仓均价
	OpenCostLong   float64 `json:"open_cost_long" csv:"open_cost_long"`     // 多头开仓市值
	OpenCostShort  float64 `json:"open_cost_short" csv:"open_cost_short"`   // 空头开仓市值

	OpenCostLongHis    float64 `json:"open_cost_long_his" csv:"open_cost_long_his"`       // 多头开仓市值(昨仓)
	OpenCostLongToday  float64 `json:"open_cost_long_today" csv:"open_cost_long_today"`   // 多头开仓市值(今仓)
	OpenCostShortToday float64 `json:"open_cost_short_today" csv:"open_cost_short_today"` // 空头开仓市值(今仓)
	OpenCostShortHis   float64 `json:"open_cost_short_his" csv:"open_cost_short_his"`     // 空头开仓市值(昨仓)

	PositionPriceLong  float64 `json:"position_price_long" csv:"position_price_long"`   // 多头持仓均价
	PositionPriceShort float64 `json:"position_price_short" csv:"position_price_short"` // 空头持仓均价

	PositionCostLong  float64 `json:"position_cost_long" csv:"position_cost_long"`   // 多头持仓成本
	PositionCostShort float64 `json:"position_cost_short" csv:"position_cost_short"` // 空头持仓成本

	PositionCostLongToday float64 `json:"position_cost_long_today" csv:"position_cost_long_today"` // 多头持仓成本(今仓)
	PositionCostLongHis   float64 `json:"position_cost_long_his" csv:"position_cost_long_his"`     // 多头持仓成本(昨仓)

	PositionCostShortToday float64 `json:"position_cost_short_today" csv:"position_cost_short_today"` // 空头持仓成本(今仓)
	PositionCostShortHis   float64 `json:"position_cost_short_his" csv:"position_cost_short_his"`     // 空头持仓成本(昨仓)

	FloatProfitLong  float64 `json:"float_profit_long" csv:"float_profit_long"`   // 多头浮动盈亏
	FloatProfitShort float64 `json:"float_profit_short" csv:"float_profit_short"` // 空头浮动盈亏
	FloatProfit      float64 `json:"float_profit" csv:"float_profit"`             // 浮动盈亏

	PositionProfitLong  float64 `json:"position_profit_long" csv:"position_profit_long"`   // 多头持仓盈亏
	PositionProfitShort float64 `json:"position_profit_short" csv:"position_profit_short"` // 空头持仓盈亏
	PositionProfit      float64 `json:"position_profit" csv:"position_profit"`             // 持仓盈亏

	// 保证金占用
	MarginLong  float64 `json:"margin_long" csv:"margin_long"`   // 多头保证金占用
	MarginShort float64 `json:"margin_short" csv:"margin_short"` // 空头保证金占用

	MarginLongToday  float64 `json:"margin_long_today" csv:"margin_long_today"`   // 多头保证金占用(今仓)
	MarginShortToday float64 `json:"margin_short_today" csv:"margin_short_today"` // 空头保证金占用(今仓)

	MarginLongHis  float64 `json:"margin_long_his" csv:"margin_long_his"`   // 多头保证金占用(昨仓)
	MarginShortHis float64 `json:"margin_short_his" csv:"margin_short_his"` // 空头保证金占用(昨仓)

	Margin       float64 `json:"margin" csv:"margin"`               // 保证金占用
	FrozenMargin float64 `json:"frozen_margin" csv:"frozen_margin"` // 冻结保证金

	LastPrice          float64
	PreSettlementPrice float64
}

func (ps *Position) LongAvailable() int {
	return ps.VolumeLong - ps.VolumeLongFrozen
}

func (ps *Position) ShortAvailable() int {
	return ps.VolumeLong - ps.VolumeLongFrozen
}

func (ps *Position) CalcProfit(ct *Contract) {
	ps.VolumeLong = ps.VolumeLongTd + ps.VolumeLongHis
	ps.VolumeShort = ps.VolumeShortTd + ps.VolumeShortHis
	ps.VolumeLongFrozen = ps.VolumeLongFrozenTd + ps.VolumeLongFrozenHis
	ps.VolumeShortFrozen = ps.VolumeShortFrozenTd + ps.VolumeShortFrozenHis
	ps.Margin = ps.MarginLong + ps.MarginShort

	if !IsValid(ps.LastPrice) {
		ps.LastPrice = ps.PreSettlementPrice
	}

	ps.PositionProfitLong = ps.LastPrice*float64(ps.VolumeLong*ct.VolumeMultiple) - ps.PositionCostLong
	ps.PositionProfitShort = ps.PositionCostShort - ps.LastPrice*float64(ps.VolumeShort)*float64(ct.VolumeMultiple)
	ps.PositionProfit = ps.PositionProfitLong + ps.PositionProfitShort

	ps.FloatProfitLong = ps.LastPrice*float64(ps.VolumeLong*ct.VolumeMultiple) - ps.OpenCostLong
	ps.FloatProfitShort = ps.OpenCostShort - ps.LastPrice*float64(ps.VolumeShort)*float64(ct.VolumeMultiple)
	ps.FloatProfit = ps.FloatProfitLong + ps.FloatProfitShort

	if ps.VolumeLong > 0 {
		ps.OpenPriceLong = ps.OpenCostLong / float64(ps.VolumeLong*ct.VolumeMultiple)
		ps.PositionPriceLong = ps.PositionCostLong / float64(ps.VolumeLong*ct.VolumeMultiple)
	} else {
		ps.OpenPriceLong = 0
		ps.PositionPriceLong = 0
	}
	if ps.VolumeShort > 0 {
		ps.OpenPriceShort = ps.OpenCostShort / float64(ps.VolumeShort*ct.VolumeMultiple)
		ps.PositionPriceShort = ps.PositionCostShort / float64(ps.VolumeShort*ct.VolumeMultiple)

	} else {
		ps.OpenPriceShort = 0
		ps.PositionPriceShort = 0
	}
}

type Contract struct {
	Ins                  string                           `json:"ins,omitempty" csv:"ins,omitempty"`
	Name                 string                           `json:"name,omitempty" csv:"name,omitempty"`
	Exchange             string                           `json:"exchange,omitempty" csv:"exchange,omitempty"`
	ProductClass         thost.TThostFtdcProductClassType `json:"product_class,omitempty" csv:"product_class,omitempty"`
	MaxMarketOrderVolume int32                            `json:"max_market_order_volume,omitempty" csv:"max_market_order_volume,omitempty"`
	MinMarketOrderVolume int32                            `json:"min_market_order_volume,omitempty" csv:"min_market_order_volume,omitempty"`
	MaxLimitOrderVolume  int32                            `json:"max_limit_order_volume,omitempty" csv:"max_limit_order_volume,omitempty"`
	MinLimitOrderVolume  int32                            `json:"min_limit_order_volume,omitempty" csv:"min_limit_order_volume,omitempty"`
	LongMarginRatio      float64                          `json:"long_margin_ratio,omitempty" csv:"long_margin_ratio,omitempty"`
	ShortMarginRatio     float64                          `json:"short_margin_ratio,omitempty" csv:"short_margin_ratio,omitempty"`
	VolumeMultiple       int                              `json:"volume_multiple,omitempty" csv:"volume_multiple,omitempty"`
	PriceTick            float64                          `json:"price_tick,omitempty" csv:"price_tick,omitempty"`
}

func FromInstrument(ins *thost.CThostFtdcInstrumentField, contract ...*Contract) *Contract {
	var c *Contract
	if len(contract) == 0 {
		c = &Contract{}
	} else {
		c = contract[0]
	}

	c.Ins = ins.InstrumentID.String()
	c.Name = ins.InstrumentName.GBString()
	c.Exchange = ins.ExchangeID.String()
	c.ProductClass = ins.ProductClass
	c.MaxMarketOrderVolume = int32(ins.MaxMarketOrderVolume)
	c.MinMarketOrderVolume = int32(ins.MinMarketOrderVolume)
	c.MaxLimitOrderVolume = int32(ins.MaxLimitOrderVolume)
	c.MinLimitOrderVolume = int32(ins.MinLimitOrderVolume)
	c.LongMarginRatio = float64(ins.LongMarginRatio)
	c.ShortMarginRatio = float64(ins.ShortMarginRatio)
	c.VolumeMultiple = int(ins.VolumeMultiple)
	c.PriceTick = float64(ins.PriceTick)

	return c
}
