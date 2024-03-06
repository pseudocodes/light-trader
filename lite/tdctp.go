package lite

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/pseudocodes/go2ctp/ctp"
	"github.com/pseudocodes/go2ctp/thost"
	"github.com/spf13/cast"
)

var (
	Exchanges = []string{"SHFE", "INE", "DCE", "CZCE", "CFFEX", "GFEX"}
)

type TdCtp struct {
	ctp.BaseTraderSpi
	trader *Trader

	bLogin atomic.Bool
	bInit  atomic.Bool
	initC  chan int
	tdapi  thost.TraderApi
}

func CreateTdCtp(trader *Trader) *TdCtp {
	s := &TdCtp{
		trader: trader,
		initC:  make(chan int),
	}
	s.tdapi = ctp.CreateTraderApi(ctp.TraderFlowPath("./tdcons/"))
	return s
}

func (s *TdCtp) Connect() error {
	s.tdapi.RegisterSpi(s)
	s.tdapi.SubscribePublicTopic(thost.THOST_TERT_QUICK)
	s.tdapi.SubscribePrivateTopic(thost.THOST_TERT_QUICK)

	for _, front := range s.trader.config.TdFronts {
		s.tdapi.RegisterFront(front)
	}
	s.tdapi.RegisterSpi(s)
	s.tdapi.Init()

	select {
	case <-time.After(60 * time.Second):
		return fmt.Errorf("td connect timeout")
	case r := <-s.initC:
		if r != 0 {
			return fmt.Errorf("tdapi init error: %d", r)
		}
	}
	return nil
}

func (s *TdCtp) Close() {
	s.tdapi.Release()
}

// /当客户端与交易后台建立起通信连接时（还未登录前），该方法被调用。
func (s *TdCtp) OnFrontConnected() {
	log.Println("on_front_connected")

	ret := s.ReqAuth()
	if ret < 0 {
		log.Printf("tdapi req auth error: %v\n", ret)
		s.initC <- ret
	}

}

// /当客户端与交易后台通信连接断开时，该方法被调用。当发生这个情况后，API会自动重新连接，客户端可不做处理。
// /@param nReason 错误原因
// /        0x1001 网络读失败
// /        0x1002 网络写失败
// /        0x2001 接收心跳超时
// /        0x2002 发送心跳失败
// /        0x2003 收到错误报文
func (s *TdCtp) OnFrontDisconnected(nReason int) {
	log.Printf("OnFrontDisconnected: %v\n", nReason)
}

// /客户端认证响应
func (s *TdCtp) OnRspAuthenticate(pRspAuthenticateField *thost.CThostFtdcRspAuthenticateField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Td OnRspAuthenticate[%s:%s] error: %s\n", pRspAuthenticateField.UserID.String(), pRspAuthenticateField.BrokerID.String(), pRspInfo.ErrorMsg.GBString())
		s.initC <- int(pRspInfo.ErrorID)
		return
	}

	if bIsLast {
		ret := s.ReqUserLogin()
		if ret < 0 {
			log.Printf("tdapi req user login error: %v\n", ret)
			s.initC <- ret
		}
	}
}

// /登录请求响应
func (s *TdCtp) OnRspUserLogin(pRspUserLogin *thost.CThostFtdcRspUserLoginField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("td OnRspUserLogin[%s:%s] error: %s\n", pRspUserLogin.UserID.String(), pRspUserLogin.BrokerID.String(), pRspInfo.ErrorMsg.GBString())
		return
	}
	if bIsLast {
		s.bLogin.Store(true)
		s.trader.sessionId = int32(pRspUserLogin.SessionID)
		s.trader.frontId = int32(pRspUserLogin.FrontID)
		s.trader.orderRef = cast.ToInt64(pRspUserLogin.MaxOrderRef.String())

		log.Printf("OnRspUserLogin SysVersion[%s]\n", pRspUserLogin.SysVersion.GBString())
		ret := s.ReqSettlementConfirm()
		if ret < 0 {
			log.Printf("tdapi req settlement confirm error: %v\n", ret)
			s.initC <- ret
		}
	}
}

// /报单录入请求响应
func (s *TdCtp) OnRspOrderInsert(pInputOrder *thost.CThostFtdcInputOrderField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Td OnRspOrderInsert[%v] error: %s\n", pRspInfo.ErrorID, pRspInfo.ErrorMsg.GBString())
		return
	}
}

// /报单操作请求响应
func (s *TdCtp) OnRspOrderAction(pInputOrderAction *thost.CThostFtdcInputOrderActionField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Td OnRspOrderAction[%v] error: %s\n", pRspInfo.ErrorID, pRspInfo.ErrorMsg.GBString())
		return
	}
}

// /投资者结算结果确认响应
func (s *TdCtp) OnRspSettlementInfoConfirm(pSettlementInfoConfirm *thost.CThostFtdcSettlementInfoConfirmField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Td OnRspSettlementInfoConfirm[%s:%s] error: %s\n", pSettlementInfoConfirm.InvestorID.String(), pSettlementInfoConfirm.BrokerID.String(), pRspInfo.ErrorMsg.GBString())
		return
	}
	if pSettlementInfoConfirm != nil {
		// ConfirmDate TThostFtdcDateType
		//  确认时间
		// ConfirmTime TThostFtdcTimeType
		//  结算编号
		// SettlementID TThostFtdcSettlementIDType
		log.Printf("%s %s [%d]\n", pSettlementInfoConfirm.ConfirmDate.String(), pSettlementInfoConfirm.ConfirmTime.String(), pSettlementInfoConfirm.SettlementID)
	}
	if bIsLast {
		if ret := s.ReqQryAccount(); ret < 0 {
			log.Printf("tdapi req settlement confirm error: %v\n", ret)
			s.initC <- ret
		}
	}
}

// /请求查询报单响应
func (s *TdCtp) OnRspQryOrder(pOrder *thost.CThostFtdcOrderField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Td OnRspQryOrder[%v] error: %s\n", pRspInfo.ErrorID, pRspInfo.ErrorMsg.GBString())
		return
	}
	if pOrder != nil {
		order := OrderFrom(&Order{}, pOrder)
		// log.Printf("OnRspQryOrder -> : %s\n", dumpOrderToText(order))
		s.trader.OrderMap.Store(order.OrderID, order)
		if order.VolumeLeft > 0 {
			s.trader.InputOrderSet.Add(order.OrderID)
		}
		// s.trader.portfolio.UpdateOrder(order)
	}

	if bIsLast {
		log.Println("OnRspQryOrder finished")
		s.bInit.Store(true)
		// s.initC <- 0
		time.Sleep(1000 * time.Millisecond)
		s.ReqQryClassifiedInstrument("", "", thost.THOST_FTDC_INS_FUTURE, thost.THOST_FTDC_TD_TRADE)
	}
}

// /请求查询成交响应
func (s *TdCtp) OnRspQryTrade(pTrade *thost.CThostFtdcTradeField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	panic("not implemented") // TODO: Implement
}

// /请求查询投资者持仓响应
func (s *TdCtp) OnRspQryInvestorPosition(pInvestorPosition *thost.CThostFtdcInvestorPositionField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Td OnRspQryInvestorPosition[%s:%s] error: %s\n", pInvestorPosition.InvestorID.String(), pInvestorPosition.BrokerID.String(), pRspInfo.ErrorMsg.GBString())
		return
	}
	if pInvestorPosition != nil {
		s.trader.portfolio.UpdatePosi(pInvestorPosition)
	}

	if bIsLast {
		time.Sleep(1100 * time.Millisecond)
		s.ReqQryOrder()
	}
}

// /请求查询资金账户响应
func (s *TdCtp) OnRspQryTradingAccount(pTradingAccount *thost.CThostFtdcTradingAccountField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Td OnRspQryTradingAccount[%v] error: %s\n", pRspInfo.ErrorID, pRspInfo.ErrorMsg.GBString())
		return
	}

	s.trader.portfolio.UpdateAccount(pTradingAccount)
	log.Printf("Balance[%v] Available[%v]\n", pTradingAccount.Balance, pTradingAccount.Available)
	if bIsLast {
		if ret := s.ReqQryPosition(); ret < 0 {
			log.Printf("tdapi req settlement confirm error: %v\n", ret)
			s.initC <- ret
		}
	}
}

// /请求查询合约保证金率响应
func (s *TdCtp) OnRspQryInstrumentMarginRate(pInstrumentMarginRate *thost.CThostFtdcInstrumentMarginRateField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Td OnRspQryInstrumentMarginRate[%v] error: %s\n", pRspInfo.ErrorID, pRspInfo.ErrorMsg.GBString())
		return
	}
	info := pInstrumentMarginRate
	if info != nil {
		log.Printf("ins[%v.%v] HedgeFlag[%v]\n", info.InstrumentID.String(), info.ExchangeID.String(), info.HedgeFlag.String())
	}
	if bIsLast {
		log.Println("OnRspQryInstrumentMarginRate finish")
	}
}

// /请求查询合约手续费率响应
func (s *TdCtp) OnRspQryInstrumentCommissionRate(pInstrumentCommissionRate *thost.CThostFtdcInstrumentCommissionRateField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Td OnRspQryInstrumentCommissionRate[%v] error: %s\n", pRspInfo.ErrorID, pRspInfo.ErrorMsg.GBString())
		return
	}
	if pInstrumentCommissionRate != nil {
		info := pInstrumentCommissionRate
		log.Printf("ins[%v.%v] %v OpenRatioByMoney[%.4f] OpenRatioByVolume[%.4f] CloseRatioByMoney[%.4f], CloseRatioByVolume[%.4f], CloseTodayRatioByMoney[%.4f], CloseTodayRatioByVolume[%.4f],\n",
			info.InstrumentID.String(), info.ExchangeID.String(), info.InvestorID.String(),
			info.OpenRatioByMoney, info.OpenRatioByVolume,
			info.CloseRatioByMoney, info.CloseRatioByVolume,
			info.CloseTodayRatioByMoney, info.CloseTodayRatioByVolume,
		)
	}
	if bIsLast {
		log.Println("OnRspQryInstrumentCommissionRate finish")
	}
}

// /请求查询合约响应
func (s *TdCtp) OnRspQryInstrument(pInstrument *thost.CThostFtdcInstrumentField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
}

// /请求查询行情响应
func (s *TdCtp) OnRspQryDepthMarketData(pDepthMarketData *thost.CThostFtdcDepthMarketDataField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
}

// /请求查询投资者持仓明细响应
func (s *TdCtp) OnRspQryInvestorPositionDetail(pInvestorPositionDetail *thost.CThostFtdcInvestorPositionDetailField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
}

// 请求查询分类合约响应
func (s *TdCtp) OnRspQryClassifiedInstrument(pInstrument *thost.CThostFtdcInstrumentField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Td OnRspQryClassifiedInstrument[%v] error: %s\n", pRspInfo.ErrorID, pRspInfo.ErrorMsg.GBString())
		return
	}

	if pInstrument != nil {
		// log.Printf("ins[%v.%v] price_tick[%.2f] vol_multi[%v]\n", pInstrument.InstrumentID.String(), pInstrument.ExchangeID.String(), pInstrument.PriceTick, pInstrument.VolumeMultiple)
		s.trader.setContract(pInstrument)
	}

	if bIsLast {
		s.initC <- 0
		log.Println("OnRspQryClassifiedInstrument finished")
	}
}

// /错误应答
func (s *TdCtp) OnRspError(pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Td OnRspError[%v] error: %s\n", pRspInfo.ErrorID, pRspInfo.ErrorMsg.GBString())
		return
	}
}

// /报单通知
func (s *TdCtp) OnRtnOrder(pOrder *thost.CThostFtdcOrderField) {
	if pOrder == nil {
		return
	}
	orderId := GetOrderKey(int32(pOrder.FrontID), int32(pOrder.SessionID), pOrder.OrderRef.String())
	rtnOrder := OrderFrom(&Order{}, pOrder)
	ins := rtnOrder.InstrumentID
	// order, ok := s.trader.OrderMap.Load(orderId)
	s.trader.OrderMap.Store(rtnOrder.OrderID, rtnOrder)

	switch pOrder.OrderStatus {
	case thost.THOST_FTDC_OST_AllTraded, thost.THOST_FTDC_OST_Canceled:
		if s.trader.InputOrderSet.Contains(orderId) {
			s.trader.InputOrderSet.Remove(orderId)
		}
		if pOrder.CombOffsetFlag[0] != byte(thost.THOST_FTDC_OF_Open) {
			// 释放冻结仓位
			s.trader.portfolio.CalcFrozen(ins, s.trader.getActiveOrders(ins))
		} else {
			//TODO 解冻资金
		}

		s.trader.liteSpi.OnOrder(rtnOrder)
	case thost.THOST_FTDC_OST_NoTradeQueueing, thost.THOST_FTDC_OST_PartTradedQueueing, thost.THOST_FTDC_OST_Touched:
		if pOrder.CombOffsetFlag[0] != byte(thost.THOST_FTDC_OF_Open) {
			// 计算冻结仓位
			s.trader.portfolio.CalcFrozen(ins, s.trader.getActiveOrders(ins))
		} else {
			//TODO: 计算冻结资金
		}
	case thost.THOST_FTDC_OST_NoTradeNotQueueing, thost.THOST_FTDC_OST_NotTouched, thost.THOST_FTDC_OST_PartTradedNotQueueing:
	case thost.THOST_FTDC_OST_Unknown:

	}
	log.Println(dumpOrderToText(rtnOrder))
}

// /成交通知
func (s *TdCtp) OnRtnTrade(pTrade *thost.CThostFtdcTradeField) {
	if pTrade == nil {
		return
	}
	s.trader.portfolio.UpdateTrade(pTrade)
	ins := pTrade.InstrumentID.String()
	posi := s.trader.GetPosition(ins)
	s.trader.liteSpi.OnPositionUpdated(posi)
}

// /报单录入错误回报
func (s *TdCtp) OnErrRtnOrderInsert(pInputOrder *thost.CThostFtdcInputOrderField, pRspInfo *thost.CThostFtdcRspInfoField) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Td OnRspError[%v] error: %s\n", pRspInfo.ErrorID, pRspInfo.ErrorMsg.GBString())
		if pInputOrder != nil {
			log.Printf("报单录入错误")
		}
		return
	}
}

// /报单操作错误回报
func (s *TdCtp) OnErrRtnOrderAction(pOrderAction *thost.CThostFtdcOrderActionField, pRspInfo *thost.CThostFtdcRspInfoField) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Td OnErrRtnOrderAction[%v] error: %s\n", pRspInfo.ErrorID, pRspInfo.ErrorMsg.GBString())
		if pOrderAction != nil {
			log.Printf("order cancel failed\n")
		}
		return
	}
}

// /合约交易状态通知
func (s *TdCtp) OnRtnInstrumentStatus(pInstrumentStatus *thost.CThostFtdcInstrumentStatusField) {
	// panic("not implemented") // TODO: Implement
}

// /交易通知
func (s *TdCtp) OnRtnTradingNotice(pTradingNoticeInfo *thost.CThostFtdcTradingNoticeInfoField) {
	// panic("not implemented") // TODO: Implement
	if pTradingNoticeInfo != nil {
		log.Printf("OnRtnTradingNotice: %s\n", pTradingNoticeInfo.FieldContent.GBString())
	}
}

func (s *TdCtp) ReqAuth() int {
	var f thost.CThostFtdcReqAuthenticateField
	copy(f.BrokerID[:], []byte(s.trader.config.BrokerID))
	copy(f.UserID[:], []byte(s.trader.config.UserID))
	copy(f.AppID[:], []byte(s.trader.config.AppID))
	copy(f.AuthCode[:], []byte(s.trader.config.AuthCode))

	reqid := s.trader.reqid.Add(1)
	r := s.tdapi.ReqAuthenticate(&f, int(reqid))
	if r != 0 {
		log.Printf("ReqAuthenticate failed: %d\n", r)
	}
	return r

}

func (s *TdCtp) ReqUserLogin() int {
	var f thost.CThostFtdcReqUserLoginField
	s.trader.reqid.Add(1)
	copy(f.BrokerID[:], []byte(s.trader.config.BrokerID))
	copy(f.UserID[:], []byte(s.trader.config.UserID))
	copy(f.Password[:], []byte(s.trader.config.Password))
	reqid := s.trader.reqid.Add(1)
	r := s.tdapi.ReqUserLogin(&f, int(reqid))
	if r != 0 {
		log.Printf("ReqUserLogin failed: %d\n", r)
	}
	return r
}

func (s *TdCtp) ReqSettlementConfirm() int {
	var f thost.CThostFtdcSettlementInfoConfirmField
	s.trader.reqid.Add(1)
	copy(f.BrokerID[:], []byte(s.trader.config.BrokerID))
	copy(f.InvestorID[:], []byte(s.trader.config.UserID))
	reqid := s.trader.reqid.Add(1)
	r := s.tdapi.ReqSettlementInfoConfirm(&f, int(reqid))
	if r != 0 {
		log.Printf("ReqSettlementInfoConfirm failed: %d\n", r)
	}
	return r
}

func (s *TdCtp) ReqQryAccount() int {
	var f thost.CThostFtdcQryTradingAccountField
	copy(f.BrokerID[:], []byte(s.trader.config.BrokerID))
	copy(f.InvestorID[:], []byte(s.trader.config.UserID))
	reqid := s.trader.reqid.Add(1)
	r := s.tdapi.ReqQryTradingAccount(&f, int(reqid))
	if r != 0 {
		log.Printf("ReqQryTradingAccount failed: %d\n", r)
	}
	return r
}

func (s *TdCtp) ReqQryPosition() int {
	var f = &thost.CThostFtdcQryInvestorPositionField{}
	copy(f.BrokerID[:], []byte(s.trader.config.BrokerID))
	copy(f.InvestorID[:], []byte(s.trader.config.UserID))
	reqid := s.trader.reqid.Add(1)
	r := s.tdapi.ReqQryInvestorPosition(f, int(reqid))
	if r != 0 {
		log.Printf("ReqQryInvestorPosition failed: %d\n", r)
	}
	return r
}

func (s *TdCtp) ReqQryOrder() int {
	var f thost.CThostFtdcQryOrderField
	copy(f.BrokerID[:], []byte(s.trader.config.BrokerID))
	copy(f.InvestorID[:], []byte(s.trader.config.UserID))
	reqid := s.trader.reqid.Add(1)
	r := s.tdapi.ReqQryOrder(&f, int(reqid))
	if r != 0 {
		log.Printf("ReqQryOrder failed: %d\n", r)
	}
	return r
}

func (s *TdCtp) ReqQryClassifiedInstrument(ins, exchange string, classType thost.TThostFtdcClassTypeType, tradingType thost.TThostFtdcTradingTypeType) int {
	var f thost.CThostFtdcQryClassifiedInstrumentField
	copy(f.InstrumentID[:], []byte(ins))
	copy(f.ExchangeID[:], []byte(exchange))
	f.ClassType = classType
	f.TradingType = tradingType
	reqid := s.trader.reqid.Add(1)
	// time.Sleep(1000) // flow control
	r := s.tdapi.ReqQryClassifiedInstrument(&f, int(reqid))
	if r != 0 {
		log.Printf("ReqQryClassifiedInstrument failed: %d\n", r)
	}
	return r
}

func (s *TdCtp) ReqQryInstrumentCommissionRate(ins, exchange string) int {
	var f thost.CThostFtdcQryInstrumentCommissionRateField
	copy(f.BrokerID[:], []byte(s.trader.config.BrokerID))
	copy(f.InvestorID[:], []byte(s.trader.config.UserID))
	copy(f.InstrumentID[:], []byte(ins))
	copy(f.ExchangeID[:], []byte(exchange))
	reqid := s.trader.reqid.Add(1)
	r := s.tdapi.ReqQryInstrumentCommissionRate(&f, int(reqid))
	if r != 0 {
		log.Printf("ReqQryInstrumentCommissionRate failed: %d\n", r)
	}
	return r
}

func (s *TdCtp) ReqQryInstrumentMarginRate(ins, exchange string) int {
	var f thost.CThostFtdcQryInstrumentMarginRateField
	copy(f.BrokerID[:], []byte(s.trader.config.BrokerID))
	copy(f.InvestorID[:], []byte(s.trader.config.UserID))
	copy(f.InstrumentID[:], []byte(ins))
	copy(f.ExchangeID[:], []byte(exchange))
	f.HedgeFlag = thost.THOST_FTDC_HF_Speculation

	reqid := s.trader.reqid.Add(1)
	r := s.tdapi.ReqQryInstrumentMarginRate(&f, int(reqid))
	if r != 0 {
		log.Printf("ReqQryInstrumentMarginRate failed: %d\n", r)
	}
	return r
}
