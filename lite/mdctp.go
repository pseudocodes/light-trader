package lite

import (
	"fmt"
	"log"
	"time"

	"github.com/pseudocodes/go2ctp/ctp"
	"github.com/pseudocodes/go2ctp/thost"
)

type MdCtp struct {
	ctp.BaseMdSpi
	trader *Trader
	loginC chan int
	mdapi  thost.MdApi
}

func CreateMdCtp(trader *Trader) *MdCtp {
	c := &MdCtp{
		trader: trader,
		loginC: make(chan int),
	}
	c.mdapi = ctp.CreateMdApi(ctp.MdFlowPath("./mdcons/"), ctp.MdUsingUDP(false), ctp.MdMultiCast(false))
	return c
}

func (c *MdCtp) Connect() error {
	for _, front := range c.trader.config.MdFronts {
		c.mdapi.RegisterFront(front)
	}
	c.mdapi.RegisterSpi(c)
	c.mdapi.Init()

	select {
	case <-time.After(15 * time.Second):
		return fmt.Errorf("md connect timeout")
	case r := <-c.loginC:
		if r != 0 {
			return fmt.Errorf("mdapi connect error: %d", r)
		}
	}
	return nil
}

func (c *MdCtp) Close() {
	c.mdapi.Release()
}

func (c *MdCtp) OnFrontConnected() {
	log.Println("on_front_connected")
	var f thost.CThostFtdcReqUserLoginField

	ret := c.mdapi.ReqUserLogin(&f, 0)
	if ret < 0 {
		log.Printf("mdapi login error: %v\n", ret)
		c.loginC <- ret
	}
}

// /当客户端与交易后台通信连接断开时，该方法被调用。当发生这个情况后，API会自动重新连接，客户端可不做处理。
// /@param nReason 错误原因
// /        0x1001 网络读失败
// /        0x1002 网络写失败
// /        0x2001 接收心跳超时
// /        0x2002 发送心跳失败
// /        0x2003 收到错误报文
func (c *MdCtp) OnFrontDisconnected(nReason int) {
	log.Printf("OnFrontDisconnected: %v\n", nReason)
}

// /登录请求响应
func (c *MdCtp) OnRspUserLogin(pRspUserLogin *thost.CThostFtdcRspUserLoginField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("Md OnRspUserLogin[%s:%s] error: %s\n", pRspUserLogin.UserID.String(), pRspUserLogin.BrokerID.String(), pRspInfo.ErrorMsg.GBString())
		return
	}
	if bIsLast {
		c.loginC <- 0
	}
}

// /错误应答
func (c *MdCtp) OnRspError(pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("OnRspError: %s\n", pRspInfo.ErrorMsg.GBString())
	}
}

// /订阅行情应答
func (c *MdCtp) OnRspSubMarketData(pSpecificInstrument *thost.CThostFtdcSpecificInstrumentField, pRspInfo *thost.CThostFtdcRspInfoField, nRequestID int, bIsLast bool) {
	if pRspInfo != nil && pRspInfo.ErrorID != 0 {
		log.Printf("OnRspSubMarketData[%s] error: %s\n", pSpecificInstrument.InstrumentID.GBString(), pRspInfo.ErrorMsg.GBString())
		return
	}
}

// /深度行情通知
func (c *MdCtp) OnRtnDepthMarketData(pDepthMarketData *thost.CThostFtdcDepthMarketDataField) {
	if c.OnRtnDepthMarketDataCallback != nil {
		c.OnRtnDepthMarketDataCallback(pDepthMarketData)
	}
	c.trader.liteSpi.OnData(pDepthMarketData)
}

func (c *MdCtp) Subscribe(instruments ...string) int {
	return c.mdapi.SubscribeMarketData(instruments...)
}
