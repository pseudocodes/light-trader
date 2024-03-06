package lite

import "github.com/pseudocodes/go2ctp/thost"

type LiteSpi interface {
	OnData(*thost.CThostFtdcDepthMarketDataField)
	OnOrder(*Order)
	OnTrade(*thost.CThostFtdcTradeField)
	OnPositionUpdated(*Position)
}

type BaseLiteSpi struct {
}

func (l *BaseLiteSpi) OnData(_ *thost.CThostFtdcDepthMarketDataField) {
}

func (l *BaseLiteSpi) OnOrder(_ *Order) {
}

func (l *BaseLiteSpi) OnTrade(_ *thost.CThostFtdcTradeField) {
}

func (l *BaseLiteSpi) OnPositionUpdated(*Position) {
}

type MarketMakingStrategy struct {
	BaseLiteSpi
	engine *Trader
}

func NewMarketMakingStrategy(cfg *Config) *MarketMakingStrategy {
	return nil
}

func (s *MarketMakingStrategy) Setup() error {
	s.engine.RegisterLiteSpi(s)
	return nil
}

func (s *MarketMakingStrategy) Run() error {
	s.engine.Start()
	return nil
}

func (s *MarketMakingStrategy) OnData(tick *thost.CThostFtdcDepthMarketDataField) {

}

func (s *MarketMakingStrategy) OnOrder(order *Order) {
}

func (s *MarketMakingStrategy) OnTrade(fill *thost.CThostFtdcTradeField) {

}
