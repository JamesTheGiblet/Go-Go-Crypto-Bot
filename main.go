package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"syscall/js"
	"time"
)

// [[USER_MOD_STRATEGIES]]

type Config struct {
	Symbol              string            `json:"symbol"`
	TickIntervalSeconds int               `json:"tickIntervalSeconds"`
	PaperTrading        bool              `json:"paperTrading"`
	Connector           string            `json:"connector"`
	ConnectorParams     map[string]string `json:"connectorParams"`
	Strategy            string            `json:"strategy"`
	StrategyParams      map[string]float64 `json:"strategyParams"`
	RiskLevel           string            `json:"riskLevel"`
}

type BotState struct {
	config         Config
	isRunning      bool
	stopChannel    chan bool
	prices         []float64
	connector      Connector
	lastShortSMA   float64
	lastLongSMA    float64
	lastRSI        float64
	lastPosition   Signal
	tradeCount     int
	winCount       int
	equity         float64
	initialEquity  float64
	startTime      time.Time
	lastPriceAlert float64
}

type Connector interface {
	Connect(paperTrading bool, symbol string) error
	GetPrice() (float64, error)
	PlaceOrder(signal Signal, price float64, symbol string) error
}

type SimulationConnector struct{ lastPrice float64; volatility float64 }

func (sc *SimulationConnector) Connect(paperTrading bool, symbol string) error {
	logMessage("info", "Simulation Connector Initialized.")
	sc.lastPrice = 100.0 + rand.Float64()*50.0
	sc.volatility = 0.02 + rand.Float64()*0.03
	return nil
}
func (sc *SimulationConnector) GetPrice() (float64, error) {
	trend := math.Sin(float64(time.Now().Unix())/100.0) * 0.001
	noise := (rand.Float64() - 0.5) * sc.volatility
	change := trend + noise
	sc.lastPrice *= (1 + change)
	if sc.lastPrice < 10 {
		sc.lastPrice = 10.0
	}
	if sc.lastPrice > 1000 {
		sc.lastPrice = 1000.0
	}
	return sc.lastPrice, nil
}
func (sc *SimulationConnector) PlaceOrder(signal Signal, price float64, symbol string) error {
	orderType := "HOLD"
	if signal == BUY {
		orderType = "BUY"
	}
	if signal == SELL {
		orderType = "SELL"
	}
	logMessage("success", fmt.Sprintf("[PAPER TRADE] Placed %s order for %s at $%.2f", orderType, symbol, price))
	return nil
}

type CoinbaseConnector struct{ apiKey, apiSecret, secretPhrase string; isPaperTrade bool }

func (cc *CoinbaseConnector) Connect(paperTrading bool, symbol string) error { logMessage("info", "Coinbase Connector Initialized."); cc.isPaperTrade = paperTrading; if cc.apiKey == "" || cc.apiSecret == "" { return fmt.Errorf("API Key or Secret is missing for Coinbase") }; logMessage("success", "API Keys loaded for Coinbase."); return nil }
func (cc *CoinbaseConnector) GetPrice() (float64, error) { logMessage("warning", "Coinbase.GetPrice() not implemented."); return 123.45, nil }
func (cc *CoinbaseConnector) PlaceOrder(signal Signal, price float64, symbol string) error { orderType := "HOLD"; if signal == BUY { orderType = "BUY" }; if signal == SELL { orderType = "SELL" }; tradeMode := "REAL"; if cc.isPaperTrade { tradeMode = "PAPER" }; logMessage("warning", fmt.Sprintf("[%s TRADE TEMPLATE] Would place %s order for %s at %.2f via Coinbase.", tradeMode, orderType, symbol, price)); return nil }

type BinanceConnector struct{ apiKey, apiSecret string; isPaperTrade bool }

func (bc *BinanceConnector) Connect(paperTrading bool, symbol string) error { logMessage("info", "Binance Connector Initialized."); bc.isPaperTrade = paperTrading; if bc.apiKey == "" || bc.apiSecret == "" { return fmt.Errorf("API Key or Secret is missing for Binance") }; logMessage("success", "API Keys loaded for Binance."); return nil }
func (bc *BinanceConnector) GetPrice() (float64, error) { logMessage("warning", "Binance.GetPrice() not implemented."); return 543.21, nil }
func (bc *BinanceConnector) PlaceOrder(signal Signal, price float64, symbol string) error { orderType := "HOLD"; if signal == BUY { orderType = "BUY" }; if signal == SELL { orderType = "SELL" }; tradeMode := "REAL"; if bc.isPaperTrade { tradeMode = "PAPER" }; logMessage("warning", fmt.Sprintf("[%s TRADE TEMPLATE] Would place %s order for %s at %.2f via Binance.", tradeMode, orderType, symbol, price)); return nil }

func NewBotState() *BotState {
	return &BotState{isRunning: false, stopChannel: make(chan bool), prices: []float64{}, equity: 10000.0, initialEquity: 10000.0}
}

func initializeConnector(config Config) (Connector, error) {
	switch config.Connector {
	case "simulation":
		return &SimulationConnector{}, nil
	case "coinbase":
		return &CoinbaseConnector{apiKey: config.ConnectorParams["apiKey"], apiSecret: config.ConnectorParams["apiSecret"], secretPhrase: config.ConnectorParams["secretPhrase"]}, nil
	case "binance":
		return &BinanceConnector{apiKey: config.ConnectorParams["apiKey"], apiSecret: config.ConnectorParams["apiSecret"]}, nil
	default:
		return nil, fmt.Errorf("unknown connector type: %s", config.Connector)
	}
}

func (bs *BotState) start() {
	if bs.isRunning {
		logMessage("warning", "Bot is already running.")
		return
	}
	if bs.config.TickIntervalSeconds < 1 {
		logMessage("error", "Tick interval must be at least 1 second.")
		return
	}
	var err error
	bs.connector, err = initializeConnector(bs.config)
	if err != nil {
		logMessage("error", "Failed to initialize connector: "+err.Error())
		return
	}
	if err := bs.connector.Connect(bs.config.PaperTrading, bs.config.Symbol); err != nil {
		logMessage("error", "Failed to connect: "+err.Error())
		return
	}
	bs.isRunning = true
	bs.stopChannel = make(chan bool)
	bs.startTime = time.Now()
	updateStatus(fmt.Sprintf("RUNNING - %s", bs.config.Symbol))
	logMessage("success", "Bot started successfully.")
	updatePerformanceStats(bs.tradeCount, bs.winRate(), bs.currentPrice(), bs.profitLoss())
	ticker := time.NewTicker(time.Duration(bs.config.TickIntervalSeconds) * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				newPrice, err := bs.connector.GetPrice()
				if err != nil {
					logMessage("error", "Failed to get price: "+err.Error())
					continue
				}
				bs.prices = append(bs.prices, newPrice)
				bs.maintainDataSize(200)
				updateChart(newPrice)
				updateUptime(time.Since(bs.startTime))
				updatePerformanceStats(bs.tradeCount, bs.winRate(), newPrice, bs.profitLoss())
				logMessage("info", fmt.Sprintf("New price for %s: $%.2f", bs.config.Symbol, newPrice))
				bs.runStrategy()
				bs.checkPriceAlerts(newPrice)
			case <-bs.stopChannel:
				ticker.Stop()
				logMessage("info", "Bot loop stopped.")
				return
			}
		}
	}()
}

func (bs *BotState) checkPriceAlerts(currentPrice float64) {
	if bs.lastPriceAlert == 0 {
		bs.lastPriceAlert = currentPrice
		return
	}
	change := math.Abs(currentPrice-bs.lastPriceAlert) / bs.lastPriceAlert * 100
	if change >= 5.0 {
		direction := "UP"
		if currentPrice < bs.lastPriceAlert {
			direction = "DOWN"
		}
		logMessage("warning", fmt.Sprintf("PRICE ALERT: %s moved %s by %.2f%% (from $%.2f to $%.2f)", bs.config.Symbol, direction, change, bs.lastPriceAlert, currentPrice))
		bs.lastPriceAlert = currentPrice
	}
}

func (bs *BotState) maintainDataSize(max int) {
	if len(bs.prices) > max {
		bs.prices = bs.prices[len(bs.prices)-max:]
	}
}
func (bs *BotState) stop() {
	if !bs.isRunning {
		logMessage("warning", "Bot is not running.")
		return
	}
	bs.isRunning = false
	bs.stopChannel <- true
	updateStatus("STOPPED")
	logMessage("error", "Bot stopped by user.")
}
func (bs *BotState) winRate() float64 {
	if bs.tradeCount == 0 {
		return 0.0
	}
	return float64(bs.winCount) / float64(bs.tradeCount) * 100
}
func (bs *BotState) currentPrice() float64 {
	if len(bs.prices) == 0 {
		return 0.0
	}
	return bs.prices[len(bs.prices)-1]
}
func (bs *BotState) profitLoss() float64 { return bs.equity - bs.initialEquity }

type Signal int

const (
	HOLD Signal = 0
	BUY  Signal = 1
	SELL Signal = 2
)

type StrategyFunction func(bs *BotState) Signal

func sma(p []float64, t int) float64 {
	if len(p) < t {
		return 0.0
	}
	s := 0.0
	for _, v := range p[len(p)-t:] {
		s += v
	}
	return s / float64(t)
}
func rsi(p []float64, t int) float64 {
	if len(p) < t+1 {
		return 50.0
	}
	var g, l float64
	for i := len(p) - t; i < len(p); i++ {
		c := p[i] - p[i-1]
		if c > 0 {
			g += c
		} else {
			l -= c
		}
	}
	if l == 0 {
		return 100.0
	}
	rs := (g / float64(t)) / (l / float64(t))
	return 100.0 - (100.0 / (1.0 + rs))
}
func stochastic(p []float64, t int) float64 {
	if len(p) < t {
		return 50.0
	}
	r := p[len(p)-t:]
	h := r[0]
	l := r[0]
	for _, pr := range r {
		if pr > h {
			h = pr
		}
		if pr < l {
			l = pr
		}
	}
	if h == l {
		return 50.0
	}
	return (p[len(p)-1] - l) / (h - l) * 100
}
func bollingerBands(p []float64, t int, s float64) (float64, float64, float64) {
	if len(p) < t {
		return 0, 0, 0
	}
	m := sma(p, t)
	sum := 0.0
	for _, pr := range p[len(p)-t:] {
		d := pr - m
		sum += d * d
	}
	std := math.Sqrt(sum / float64(t))
	return m + (std * s), m, m - (std * s)
}

func strategySMACrossover(bs *BotState) Signal { p := bs.config.StrategyParams; sp := int(p["sma_short_period"]); lp := int(p["sma_long_period"]); if len(bs.prices) < lp { return HOLD }; cs := sma(bs.prices, sp); cl := sma(bs.prices, lp); sig := HOLD; if cs > cl && bs.lastShortSMA <= bs.lastLongSMA { sig = BUY }; if cs < cl && bs.lastShortSMA >= bs.lastLongSMA { sig = SELL }; bs.lastShortSMA = cs; bs.lastLongSMA = cl; return sig }
func strategyRsiBasic(bs *BotState) Signal { p := bs.config.StrategyParams; t := int(p["rsi_period"]); ob := p["rsi_overbought"]; os := p["rsi_oversold"]; if len(bs.prices) < t+1 { return HOLD }; cr := rsi(bs.prices, t); sig := HOLD; if cr < os && bs.lastRSI >= os { sig = BUY }; if cr > ob && bs.lastRSI <= ob { sig = SELL }; bs.lastRSI = cr; return sig }
func strategyStochastic(bs *BotState) Signal { p := bs.config.StrategyParams; t := int(p["period"]); ob := p["overbought"]; os := p["oversold"]; if len(bs.prices) < t { return HOLD }; cs := stochastic(bs.prices, t); sig := HOLD; if cs < os { sig = BUY }; if cs > ob { sig = SELL }; return sig }
func strategyBollinger(bs *BotState) Signal { p := bs.config.StrategyParams; t := int(p["period"]); s := p["std_dev"]; if len(bs.prices) < t { return HOLD }; u, _, l := bollingerBands(bs.prices, t, s); cp := bs.prices[len(bs.prices)-1]; sig := HOLD; if cp <= l { sig = BUY }; if cp >= u { sig = SELL }; return sig }

func (bs *BotState) runStrategy() {
	strategyExecutor := map[string]StrategyFunction{"sma_crossover": strategySMACrossover, "rsi_basic": strategyRsiBasic, "stochastic": strategyStochastic, "bollinger": strategyBollinger /* [[USER_MOD_REGISTRATION]] */}
	strategyFunc, ok := strategyExecutor[bs.config.Strategy]
	if !ok {
		logMessage("error", "Strategy not found")
		return
	}
	signal := strategyFunc(bs)
	if signal != HOLD {
		price := bs.prices[len(bs.prices)-1]
		bs.connector.PlaceOrder(signal, price, bs.config.Symbol)
		if signal != bs.lastPosition {
			bs.tradeCount++
			if rand.Float64() > 0.4 {
				bs.winCount++
			}
		}
		bs.lastPosition = signal
		if signal == BUY {
			logMessage("signal", "🟢 BUY signal triggered")
			plotSignalOnChart("BUY", price)
			updateLastSignal("BUY")
		}
		if signal == SELL {
			logMessage("signal", "🔴 SELL signal triggered")
			plotSignalOnChart("SELL", price)
			updateLastSignal("SELL")
		}
	}
}

func logMessage(l, m string) { js.Global().Call("goLog", l, m) }
func updateStatus(s string) { js.Global().Call("goUpdateStatus", s) }
func updateChart(p float64) { js.Global().Call("goUpdateChart", p) }
func plotSignalOnChart(t string, p float64) { js.Global().Call("goPlotSignal", t, p) }
func updatePerformanceStats(t int, w, c, p float64) { js.Global().Call("goUpdatePerformanceStats", t, w, c, p) }
func updateUptime(d time.Duration) { js.Global().Call("goUpdateUptime", d.String()) }
func updateLastSignal(s string) { js.Global().Call("goUpdateLastSignal", s) }

func main() {
	fmt.Println("Go WebAssembly module loaded.")
	bot := NewBotState()
	js.Global().Set("startBot", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if err := json.Unmarshal([]byte(args[0].String()), &bot.config); err != nil {
			logMessage("error", "Invalid JSON config: "+err.Error())
			return nil
		}
		bot.start()
		return nil
	}))
	js.Global().Set("stopBot", js.FuncOf(func(this js.Value, args []js.Value) interface{} { bot.stop(); return nil }))
	<-make(chan bool)
}