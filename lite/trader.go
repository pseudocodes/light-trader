package lite

import (
	"fmt"
	"log"
	"math"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/spf13/cast"

	"github.com/pseudocodes/go2ctp/thost"
)

type Config struct {
	UserID      string
	BrokerID    string
	Password    string
	AppID       string
	AuthCode    string
	MdFronts    []string
	TdFronts    []string
	Instruments []string
}

type Trader struct {
	portfolio *Portfolio
	tdctp     *TdCtp
	mdctp     *MdCtp

	inslist []string
	liteSpi LiteSpi

	config *Config

	frontId   int32
	sessionId int32
	orderRef  int64

	reqid atomic.Int32
	// ordersOnRoad map[string]*

	InputOrderSet mapset.Set[string]
	OrderMap      xsync.MapOf[string, *Order]
	ContractMap   xsync.MapOf[string, *Contract]

	quitC chan struct{}
}

func NewTrader(config *Config) *Trader {
	t := &Trader{
		config:        config,
		ContractMap:   *xsync.NewMapOf[string, *Contract](),
		OrderMap:      *xsync.NewMapOf[string, *Order](),
		InputOrderSet: mapset.NewSet[string](),
		portfolio:     NewPortfolio(),
	}
	t.portfolio.trader = t
	return t
}

func (t *Trader) Start() error {
	if isNil(t.liteSpi) {
		t.liteSpi = &BaseLiteSpi{}
	}
	if err := t.tdctp.Connect(); err != nil {
		log.Printf("tdctp connect error: %v\n", err)
	}
	if err := t.mdctp.Connect(); err != nil {
		log.Printf("mdctp connect error: %v\n", err)
	}

	if len(t.inslist) > 0 {
		t.mdctp.mdapi.SubscribeMarketData(t.inslist...)
	}
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				// t.portfolio.CalcBalance()
			case <-t.quitC:
				return
			}
		}
	}()
	return nil
}

func (t *Trader) Stop() {
	close(t.quitC)
	//TODO: Logout ?
	// t.mdctp.mdapi.UnSubscribeMarketData(t.inslist...)
	t.mdctp.Close()
	t.tdctp.Close()
}

func (t *Trader) SetTdCtp(tdctp *TdCtp) {
	t.tdctp = tdctp
}

func (t *Trader) RegisterInstruments(ins ...string) {
	t.inslist = ins
}

func (t *Trader) RegisterLiteSpi(spi LiteSpi) {
	t.liteSpi = spi
}

func (t *Trader) Subscribe(ins ...string) {
	r := t.mdctp.mdapi.SubscribeMarketData(t.inslist...)
	if r != 0 {
		log.Printf("subscrible market data error: %v\n", r)
	}
}

func (t *Trader) BuyOpen(ins, exchange string, limit float64, vol int) (string, error) {

	return t.InsertOrder(ins, exchange, limit, vol, thost.THOST_FTDC_D_Buy, thost.THOST_FTDC_OF_Open, thost.THOST_FTDC_OPT_LimitPrice, thost.THOST_FTDC_VC_AV, thost.THOST_FTDC_TC_GFD)
}

func (t *Trader) BuyClose(ins, exchange string, limit float64, vol int, isCloseToday bool) (string, error) {
	offset := thost.THOST_FTDC_OF_Close
	if isCloseToday {
		offset = thost.THOST_FTDC_OF_CloseToday
	}
	return t.InsertOrder(ins, exchange, limit, vol, thost.THOST_FTDC_D_Buy, offset, thost.THOST_FTDC_OPT_LimitPrice, thost.THOST_FTDC_VC_AV, thost.THOST_FTDC_TC_GFD)

}

func (t *Trader) SellOpen(ins, exchange string, limit float64, vol int) (string, error) {
	return t.InsertOrder(ins, exchange, limit, vol, thost.THOST_FTDC_D_Sell, thost.THOST_FTDC_OF_Open, thost.THOST_FTDC_OPT_LimitPrice, thost.THOST_FTDC_VC_AV, thost.THOST_FTDC_TC_GFD)

}

func (t *Trader) SellClose(ins, exchange string, limit float64, vol int, isCloseToday bool) (string, error) {
	offset := thost.THOST_FTDC_OF_Close
	if isCloseToday {
		offset = thost.THOST_FTDC_OF_CloseToday
	}
	return t.InsertOrder(ins, exchange, limit, vol, thost.THOST_FTDC_D_Sell, offset, thost.THOST_FTDC_OPT_LimitPrice, thost.THOST_FTDC_VC_AV, thost.THOST_FTDC_TC_GFD)
}

func (t *Trader) InsertOrder(ins string, exchange string, limit float64, vol int, dir thost.TThostFtdcDirectionType, off thost.TThostFtdcOffsetFlagType, pt thost.TThostFtdcOrderPriceTypeType, vc thost.TThostFtdcVolumeConditionType, tc thost.TThostFtdcTimeConditionType) (string, error) {
	orderRef := atomic.AddInt64(&t.orderRef, 1)
	refStr := cast.ToString(orderRef)
	orderId := fmt.Sprintf("%s.%08x.%d", refStr, t.sessionId, t.frontId)
	var inputOrder = &thost.CThostFtdcInputOrderField{
		OrderPriceType:      pt,
		Direction:           dir,
		VolumeCondition:     vc,
		TimeCondition:       tc,
		LimitPrice:          thost.TThostFtdcPriceType(limit),
		VolumeTotalOriginal: thost.TThostFtdcVolumeType(vol),
		ContingentCondition: thost.THOST_FTDC_CC_Immediately,
		ForceCloseReason:    thost.THOST_FTDC_FCC_NotForceClose,
	}
	inputOrder.CombHedgeFlag[0] = byte(thost.THOST_FTDC_HF_Speculation)
	inputOrder.CombOffsetFlag[0] = byte(off)
	copy(inputOrder.BrokerID[:], []byte(t.config.BrokerID))
	copy(inputOrder.UserID[:], []byte(t.config.UserID))
	copy(inputOrder.InvestorID[:], []byte(t.config.UserID))
	copy(inputOrder.ExchangeID[:], []byte(exchange))
	copy(inputOrder.InstrumentID[:], []byte(ins))
	copy(inputOrder.OrderRef[:], []byte(refStr))

	reqid := t.reqid.Add(1)
	r := t.tdctp.tdapi.ReqOrderInsert(inputOrder, int(reqid))
	if r != 0 {
		key := GetOrderKey(t.frontId, t.sessionId, refStr)
		ss := key + "|" + dumpInputOrderToText(inputOrder)
		return "", fmt.Errorf("insert order [%s] error: %d", ss, r)
	}
	order := OrderFromInput(inputOrder)
	order.OrderID = orderId
	order.FrontID = t.sessionId
	order.SessionID = t.frontId
	order.OrderRef = refStr

	t.InputOrderSet.Add(orderId)
	t.OrderMap.Store(orderId, order)

	return orderId, nil
}

func (t *Trader) CancelOrder(orderId string) error {
	if order, ok := t.OrderMap.Load(orderId); ok {
		if order.Status == thost.THOST_FTDC_OST_AllTraded || order.Status == thost.THOST_FTDC_OST_Canceled {
			// no need to cancel
			return nil
		}

		var f thost.CThostFtdcInputOrderActionField
		copy(f.BrokerID[:], []byte(t.config.BrokerID))
		copy(f.UserID[:], []byte(t.config.UserID))
		copy(f.InvestorID[:], []byte(t.config.UserID))
		copy(f.OrderRef[:], []byte(order.OrderRef))
		copy(f.ExchangeID[:], []byte(order.Exchange))
		copy(f.InstrumentID[:], []byte(order.InstrumentID))
		f.FrontID = thost.TThostFtdcFrontIDType(t.frontId)
		f.SessionID = thost.TThostFtdcSessionIDType(t.sessionId)
		f.ActionFlag = thost.THOST_FTDC_AF_Delete
		f.LimitPrice = 0
		f.VolumeChange = 0
		reqid := t.reqid.Add(1)

		r := t.tdctp.tdapi.ReqOrderAction(&f, int(reqid))
		if r != 0 {
			ss := dumpOrderToText(order)
			return fmt.Errorf("cancel order [%s] error: %d", ss, r)
		}
	}
	return nil
}

func (t *Trader) GetPosition(ins string) *Position {
	posi := t.portfolio.Position(ins)
	return posi
}

func (t *Trader) setContract(pInstrument *thost.CThostFtdcInstrumentField) {
	contract := FromInstrument(pInstrument)
	t.ContractMap.Store(contract.Ins, contract)
}

func (t *Trader) isOrderLocal(order *Order) bool {
	return order.FrontID == t.frontId && order.SessionID == t.sessionId
}

func (t *Trader) getActiveOrders(ins string) []*Order {
	// inslist := t.InputOrderSet.ToSlice()
	var orders = make([]*Order, 0)
	t.OrderMap.Range(func(_ string, order *Order) bool {
		if order.InstrumentID != ins {
			return true
		}
		if order.VolumeLeft > 0 && (order.Status != thost.THOST_FTDC_OST_AllTraded && order.Status != thost.THOST_FTDC_OST_Canceled) {
			orders = append(orders, order)
		}
		return true
	})
	return orders
}

type Portfolio struct {
	Balance       float64
	Avaliable     float64
	FrozenBalance float64

	Holdings xsync.MapOf[string, *Position]

	trader *Trader
	sync.Mutex
}

func NewPortfolio() *Portfolio {
	return &Portfolio{
		Holdings: *xsync.NewMapOf[string, *Position](),
	}
}

func (p *Portfolio) UpdateTick(tick *thost.CThostFtdcDepthMarketDataField) {
	ins := tick.InstrumentID.String()

	if posi, ok := p.Holdings.Load(ins); ok {
		posi.LastPrice = float64(tick.LastPrice)
	} else {
		p.Lock()
		if _, ok := p.Holdings.Load(ins); !ok {
			posi := &Position{Instrument: ins, LastPrice: float64(tick.LastPrice)}
			p.Holdings.Store(ins, posi)
		}
		p.Unlock()
	}
}

func (p *Portfolio) UpdatePosi(f *thost.CThostFtdcInvestorPositionField) {

	var (
		ins                   = f.InstrumentID.String()
		exchange              = f.ExchangeID.String()
		bHasTdYdDistinct bool = (exchange == "SHFE") || (exchange == "INE")
	)

	p.Lock()
	posi, ok := p.Holdings.Load(ins)
	if !ok {
		posi = &Position{Instrument: ins, Exchange: exchange}
		p.Holdings.Store(posi.Instrument, posi)
	}
	if f.PosiDirection == thost.THOST_FTDC_PD_Long {
		if !bHasTdYdDistinct {
			posi.VolumeLongYd = int(f.YdPosition)
		} else {
			if f.PositionDate == thost.THOST_FTDC_PSD_History {
				posi.VolumeLongYd = int(f.YdPosition)
			}
		}
		if f.PositionDate == thost.THOST_FTDC_PSD_Today {
			posi.VolumeLongTd = int(f.Position)
			posi.VolumeLongFrozenTd = int(f.LongFrozen)
			posi.PositionCostLongToday = float64(f.PositionCost)
			posi.OpenCostLongToday = float64(f.OpenCost)
			posi.MarginLongToday = float64(f.UseMargin)

		} else {
			posi.VolumeLongHis = int(f.Position)
			posi.VolumeLongFrozenHis = int(f.LongFrozen)
			posi.PositionCostLongHis = float64(f.PositionCost)
			posi.OpenCostLongHis = float64(f.OpenCost)
			posi.MarginLongHis = float64(f.UseMargin)
		}
		posi.PositionCostLong = posi.PositionCostLongToday + posi.PositionCostLongHis
		posi.OpenCostLong = posi.OpenCostLongToday + posi.OpenCostLongHis
		posi.MarginLong = posi.MarginLongToday + posi.MarginLongHis
	} else {
		if !bHasTdYdDistinct {
			posi.VolumeShortYd = int(f.YdPosition)
		} else {
			if f.PositionDate == thost.THOST_FTDC_PSD_History {
				posi.VolumeShortYd = int(f.YdPosition)
			}
		}
		if f.PositionDate == thost.THOST_FTDC_PSD_Today {

			posi.VolumeShortTd = int(f.Position)
			posi.VolumeShortFrozenTd = int(f.ShortFrozen)
			posi.PositionCostShortToday = float64(f.PositionCost)
			posi.OpenCostShortToday = float64(f.OpenCost)
			posi.MarginShortToday = float64(f.UseMargin)

		} else {

			posi.VolumeShortHis = int(f.Position)
			posi.VolumeShortFrozenHis = int(f.ShortFrozen)
			posi.PositionCostShortHis = float64(f.PositionCost)
			posi.OpenCostShortHis = float64(f.OpenCost)
			posi.MarginShortHis = float64(f.UseMargin)
		}
		posi.PositionCostShort = posi.PositionCostShortToday + posi.PositionCostShortHis
		posi.OpenCostShort = posi.OpenCostShortToday + posi.OpenCostShortHis
		posi.MarginShort = posi.MarginShortToday + posi.MarginShortHis
	}
	p.Unlock()

}

func (p *Portfolio) CalcProfit(ct *Contract) {
	ps, ok := p.Holdings.Load(ct.Ins)
	if !ok {
		return
	}
	ps.CalcProfit(ct)
}

func (p *Portfolio) CalcFrozen(ins string, actives []*Order) {

	p.Lock()
	posi := p.Position(ins)
	if posi == nil {
		posi = &Position{Instrument: ins, Exchange: actives[0].Exchange}
		p.Holdings.Store(ins, posi)
	}
	posi.VolumeLongFrozen = 0
	posi.VolumeLongFrozenTd = 0
	posi.VolumeLongFrozenHis = 0

	posi.VolumeShortFrozen = 0
	posi.VolumeShortFrozenTd = 0
	posi.VolumeShortFrozenHis = 0

	p.Unlock()
	for _, order := range actives {
		frozen := order.VolumeLeft
		if order.Direction == thost.THOST_FTDC_D_Buy {
			if order.Offset == thost.THOST_FTDC_OF_CloseToday {
				posi.VolumeShortFrozenTd += frozen
			} else if order.Offset == thost.THOST_FTDC_OF_CloseYesterday {
				posi.VolumeShortFrozenHis += frozen
			} else {
				posi.VolumeShortFrozenTd += frozen
				if posi.VolumeShortFrozenTd > posi.VolumeShortTd {
					posi.VolumeShortFrozenHis += posi.VolumeShortFrozenTd - posi.VolumeShortTd
					posi.VolumeShortFrozenTd = posi.VolumeShortTd
				}
			}
		} else {
			if order.Offset == thost.THOST_FTDC_OF_CloseToday {
				posi.VolumeLongFrozenTd += frozen
			} else if order.Offset == thost.THOST_FTDC_OF_CloseYesterday {
				posi.VolumeLongFrozenTd += frozen
			} else {
				posi.VolumeLongFrozenTd += frozen
				if posi.VolumeLongFrozenTd > posi.VolumeLongTd {
					posi.VolumeLongFrozenHis += posi.VolumeLongFrozenTd - posi.VolumeLongTd
					posi.VolumeLongFrozenTd = posi.VolumeLongTd
				}
			}
		}
	}
	posi.VolumeLongFrozen = posi.VolumeLongFrozenTd + posi.VolumeLongFrozenHis
	posi.VolumeShortFrozen = posi.VolumeShortFrozenTd + posi.VolumeShortFrozenHis
}

func (p *Portfolio) UpdateTrade(fill *thost.CThostFtdcTradeField) {
	ins := fill.InstrumentID.String()
	exchange := fill.ExchangeID.String()
	p.Lock()
	pos := p.Position(ins)
	if pos == nil {
		pos := &Position{Instrument: ins, Exchange: exchange}
		p.Holdings.Store(ins, pos)
	}
	p.Unlock()

	if fill.OffsetFlag == thost.THOST_FTDC_OF_Open {
		if fill.Direction == thost.THOST_FTDC_D_Buy {
			pos.VolumeLong += int(fill.Volume)
			pos.VolumeLongTd += int(fill.Volume)

		} else { // SELL
			pos.VolumeShort += int(fill.Volume)
			pos.VolumeShortTd += int(fill.Volume)
		}

	} else {
		if (exchange == "SHFE" || exchange == "INE") && fill.OffsetFlag != thost.THOST_FTDC_OF_CloseToday {
			if fill.Direction == thost.THOST_FTDC_D_Buy {
				// sell close history
				pos.VolumeShortHis -= int(fill.Volume)
			} else {
				// buy close history
				pos.VolumeLongHis -= int(fill.Volume)
			}
		} else {

			if fill.Direction == thost.THOST_FTDC_D_Buy {
				// sell close today
				pos.VolumeShortTd -= int(fill.Volume)
			} else {
				// buy close today
				pos.VolumeLongTd -= int(fill.Volume)
			}
		}
		if pos.VolumeLongTd < 0 {
			pos.VolumeLongHis += pos.VolumeLongTd
			pos.VolumeLongTd = 0
		}
		if pos.VolumeLongHis < 0 {
			pos.VolumeLongTd += pos.VolumeLongHis
			pos.VolumeLongHis = 0
		}
		if pos.VolumeShortTd < 0 {
			pos.VolumeShortHis += pos.VolumeShortTd
			pos.VolumeShortTd = 0
		}
		if pos.VolumeShortHis < 0 {
			pos.VolumeShortTd += pos.VolumeShortHis
			pos.VolumeShortHis = 0
		}

	}
}

func (p *Portfolio) UpdateAccount(account *thost.CThostFtdcTradingAccountField) {
	p.Balance = float64(account.Balance)
	p.Avaliable = float64(account.Available)
	p.FrozenBalance = float64(account.FrozenCash)
}

func (f *Portfolio) CalcBalance() {

	f.Holdings.Range(func(symbol string, p *Position) bool {
		return true
	})
}

func (f *Portfolio) Position(symbol string) *Position {
	pos, _ := f.Holdings.Load(symbol)
	return pos
}

func isNil(i interface{}) bool {
	if i != nil {
		return reflect.ValueOf(i).IsNil()
	}
	return true
}

func dumpInputOrderToText(order *thost.CThostFtdcInputOrderField) string {
	input := OrderFromInput(order)
	ss1 := dumpOrderParamToText("", input)
	return ss1
}

func dumpOrderToText(order *Order) string {
	return dumpOrderParamToText(order.OrderID+"|", order)
}

func dumpOrderParamToText(prefix string, order *Order) string {
	var ss1 string
	strInstrumentId := order.Exchange + "." + order.InstrumentID
	ss1 += strInstrumentId
	ss1 += "|[" + order.ExchangeOrderID + "]"
	var strDirection string
	if order.Direction == thost.THOST_FTDC_D_Buy {
		strDirection = "Buy"
	} else {
		strDirection = "Sell"
	}
	ss1 += "|" + strDirection
	var strOffsetFlag string
	switch order.Offset {
	case thost.THOST_FTDC_OF_Open:
		strOffsetFlag = "Open"
	case thost.THOST_FTDC_OF_Close:
		strOffsetFlag = "Close"
	case thost.THOST_FTDC_OF_ForceClose:
		strOffsetFlag = "ForceClose"
	case thost.THOST_FTDC_OF_CloseToday:
		strOffsetFlag = "CloseToday"
	case thost.THOST_FTDC_OF_CloseYesterday:
		strOffsetFlag = "CloseYesterday"
	case thost.THOST_FTDC_OF_ForceOff:
		strOffsetFlag = "ForceOff"
	case thost.THOST_FTDC_OF_LocalForceClose:
		strOffsetFlag = "LocalForceClose"
	default:
		strOffsetFlag = "Unknown"
	}
	ss1 += "|" + strOffsetFlag
	switch order.PriceType {
	case thost.THOST_FTDC_OPT_AnyPrice:
		ss1 += "|AnyPrice"
	case thost.THOST_FTDC_OPT_LimitPrice:
		ss1 += "|LimitPrice:" + cast.ToString(order.LimitPrice)
	case thost.THOST_FTDC_OPT_BestPrice:
		ss1 += "|BestPrice"
	case thost.THOST_FTDC_OPT_FiveLevelPrice:
		ss1 += "|FiveLevelPrice"
	default:
		ss1 += "|Unknown"
	}
	ss1 += "|TotalOri:" + cast.ToString(order.VolumeOrigin)
	ss1 += "|VolLeft:" + cast.ToString(order.VolumeLeft)

	ss1 += "|"
	switch order.SubmitStatus {
	case thost.THOST_FTDC_OSS_Accepted:
		ss1 += "Accepted"
	case thost.THOST_FTDC_OSS_CancelRejected:
		ss1 += "CancelRejected"
	case thost.THOST_FTDC_OSS_CancelSubmitted:
		ss1 += "CancelSubmitted"
	case thost.THOST_FTDC_OSS_InsertRejected:
		ss1 += "InsertRejected"
	case thost.THOST_FTDC_OSS_InsertSubmitted:
		ss1 += "InsertSubmitted"
	case thost.THOST_FTDC_OSS_ModifyRejected:
		ss1 += "ModifyRejected"
	case thost.THOST_FTDC_OSS_ModifySubmitted:
		ss1 += "ModifySubmitted"
	default:
		ss1 += "Unknown"
	}

	switch order.Status {
	case thost.THOST_FTDC_OST_AllTraded:
		ss1 += "|AllTraded"
	case thost.THOST_FTDC_OST_PartTradedNotQueueing:
		ss1 += "|PartTradedNotQueueing"
	case thost.THOST_FTDC_OST_NoTradeNotQueueing:
		ss1 += "|NoTradeNotQueueing"
	case thost.THOST_FTDC_OST_Canceled:
		ss1 += "|Canceled"
	case thost.THOST_FTDC_OST_PartTradedQueueing:
		ss1 += "|PartTradedQueueing"
	case thost.THOST_FTDC_OST_NoTradeQueueing:
		ss1 += "|NoTradeQueueing"
	case thost.THOST_FTDC_OST_Unknown:
		ss1 += "|Unknown"
	}

	if prefix != "" {
		return prefix + " " + ss1
	}
	return ss1
}

func dumpTradeToText(trade *thost.CThostFtdcTradeField) string {
	var ss1 string
	strInstrumentId := trade.ExchangeID.String() + "." + trade.InstrumentID.String()
	ss1 += strInstrumentId
	var strDirection string
	if byte(trade.Direction) == byte(thost.THOST_FTDC_D_Buy) {
		strDirection = "Buy"
	} else {
		strDirection = "Sell"
	}
	ss1 += "|" + strDirection
	var strOffsetFlag string
	switch byte(trade.OffsetFlag) {
	case byte(thost.THOST_FTDC_OF_Open):
		strOffsetFlag = "Open"
	case byte(thost.THOST_FTDC_OF_Close):
		strOffsetFlag = "Close"
	case byte(thost.THOST_FTDC_OF_ForceClose):
		strOffsetFlag = "ForceClose"
	case byte(thost.THOST_FTDC_OF_CloseToday):
		strOffsetFlag = "CloseToday"
	case byte(thost.THOST_FTDC_OF_CloseYesterday):
		strOffsetFlag = "CloseYesterday"
	case byte(thost.THOST_FTDC_OF_ForceOff):
		strOffsetFlag = "ForceOff"
	case byte(thost.THOST_FTDC_OF_LocalForceClose):
		strOffsetFlag = "LocalForceClose"
	default:
		strOffsetFlag = "Unknown"
	}
	ss1 += "|" + strOffsetFlag

	ss1 += "|TotalOri:" + cast.ToString(trade.Volume)
	ss1 += "|Price:" + fmt.Sprintf("%.2f", trade.Price)
	ss1 += "|OrderSysID: " + trade.OrderSysID.String()

	return ss1

}

func IsValid(x float64) bool {
	return !math.IsNaN(x) && (x < 1e20) && (x > -1e20)
}
