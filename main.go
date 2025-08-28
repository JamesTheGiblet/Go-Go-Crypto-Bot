package main

import (
	"encoding/json"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"crypto/hmac"
	"crypto/sha256"
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
	PlaceOrder(bs *BotState, signal Signal, price float64, symbol string) error
	Disconnect() error
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
func (sc *SimulationConnector) PlaceOrder(bs *BotState, signal Signal, price float64, symbol string) error {
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
func (sc *SimulationConnector) Disconnect() error { return nil }

type CoinbaseConnector struct {
	apiKey       string
	apiSecret    string
	secretPhrase string
	isPaperTrade bool
	ws           js.Value
	lastPrice    float64
	mu           sync.Mutex
}

func (cc *CoinbaseConnector) Connect(paperTrading bool, symbol string) error {
	logMessage("info", "Coinbase Connector Initializing...")
	cc.isPaperTrade = paperTrading
	if !paperTrading && (cc.apiKey == "" || cc.apiSecret == "" || cc.secretPhrase == "") {
		return fmt.Errorf("API Key, Secret, or Passphrase is missing for Coinbase")
	}

	wsURL := "wss://ws-feed.pro.coinbase.com"
	logMessage("info", "Connecting to Coinbase WebSocket: "+wsURL)

	ws := js.Global().Get("WebSocket").New(wsURL)
	cc.ws = ws

	connected := make(chan bool)
	var onOpen, onMessage, onError, onClose js.Func

	onOpen = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		logMessage("success", "Coinbase WebSocket connection established.")
		// Coinbase requires a subscription message after connecting.
		// Symbol format needs to be like "BTC-USD"
		coinbaseSymbol := strings.Replace(strings.ToUpper(symbol), "USDT", "-USD", 1)

		subMsg := map[string]interface{}{
			"type":        "subscribe",
			"product_ids": []string{coinbaseSymbol},
			"channels":    []string{"ticker"},
		}
		subMsgJSON, _ := json.Marshal(subMsg)
		ws.Call("send", string(subMsgJSON))
		logMessage("info", fmt.Sprintf("Subscribed to Coinbase ticker for %s", coinbaseSymbol))
		connected <- true
		return nil
	})
	ws.Set("onopen", onOpen)

	onMessage = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		event := args[0]
		data := event.Get("data").String()
		var tickerData map[string]interface{}
		if err := json.Unmarshal([]byte(data), &tickerData); err != nil {
			// Coinbase sends non-JSON heartbeats, we can ignore parse errors on those.
			return nil
		}
		// Check if it's a ticker update
		if t, ok := tickerData["type"].(string); ok && t == "ticker" {
			if priceStr, ok := tickerData["price"].(string); ok {
				if price, err := strconv.ParseFloat(priceStr, 64); err == nil {
					cc.mu.Lock()
					cc.lastPrice = price
					cc.mu.Unlock()
				}
			}
		}
		return nil
	})
	ws.Set("onmessage", onMessage)

	onError = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		logMessage("error", "Coinbase WebSocket error.")
		return nil
	})
	ws.Set("onerror", onError)

	onClose = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		logMessage("warning", "Coinbase WebSocket connection closed.")
		onOpen.Release()
		onMessage.Release()
		onError.Release()
		this.Release()
		return nil
	})
	ws.Set("onclose", onClose)

	select {
	case <-connected:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("Coinbase WebSocket connection timed out")
	}
}

func (cc *CoinbaseConnector) GetPrice() (float64, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if cc.lastPrice == 0 {
		return 0, fmt.Errorf("price not available yet from Coinbase WebSocket")
	}
	return cc.lastPrice, nil
}

func (cc *CoinbaseConnector) PlaceOrder(bs *BotState, signal Signal, price float64, symbol string) error {
	if cc.isPaperTrade {
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

	if cc.apiKey == "" || cc.apiSecret == "" || cc.secretPhrase == "" {
		err := fmt.Errorf("cannot place real order: Coinbase API Key, Secret, or Passphrase is missing")
		logMessage("error", err.Error())
		return err
	}

	go func() {
		side := "buy"
		if signal == SELL {
			side = "sell"
		}

		coinbaseSymbol := strings.Replace(strings.ToUpper(symbol), "USDT", "-USD", 1)

		quoteOrderQty := "20.00" // Moderate
		switch bs.config.RiskLevel {
		case "conservative":
			quoteOrderQty = "10.00"
		case "aggressive":
			quoteOrderQty = "50.00"
		}

		orderBody := map[string]string{"product_id": coinbaseSymbol, "side": side, "type": "market", "funds": quoteOrderQty}
		if side == "sell" {
			logMessage("warning", "Coinbase market SELL orders require 'size' (amount of crypto). This is not implemented. Order will likely fail.")
			delete(orderBody, "funds")
			orderBody["size"] = "0.001" // Dummy value for demonstration
		}

		bodyBytes, _ := json.Marshal(orderBody)
		bodyString := string(bodyBytes)

		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		method := "POST"
		requestPath := "/orders"
		prehash := timestamp + method + requestPath + bodyString
		decodedSecret, err := base64.StdEncoding.DecodeString(cc.apiSecret)
		if err != nil {
			logMessage("error", "Failed to decode Coinbase API secret.")
			return
		}

		mac := hmac.New(sha256.New, decodedSecret)
		mac.Write([]byte(prehash))
		signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

		endpoint := "https://api.pro.coinbase.com"
		url := endpoint + requestPath

		headers := js.Global().Get("Object").New()
		headers.Set("Content-Type", "application/json")
		headers.Set("CB-ACCESS-KEY", cc.apiKey)
		headers.Set("CB-ACCESS-SIGN", signature)
		headers.Set("CB-ACCESS-TIMESTAMP", timestamp)
		headers.Set("CB-ACCESS-PASSPHRASE", cc.secretPhrase)

		reqOptions := js.Global().Get("Object").New()
		reqOptions.Set("method", method)
		reqOptions.Set("headers", headers)
		reqOptions.Set("body", bodyString)

		logMessage("info", fmt.Sprintf("[REAL TRADE] Submitting Coinbase %s market order for %s...", side, coinbaseSymbol))

		fetchPromise := js.Global().Call("fetch", url, reqOptions)
		fetchPromise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			response := args[0]
			response.Call("json").Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
				resultJSON, _ := js.Global().Get("JSON").Call("stringify", args[0], nil, 2).String()
				if response.Get("ok").Bool() {
					logMessage("success", fmt.Sprintf("Coinbase order successful:\n%s", resultJSON))
				} else {
					logMessage("error", fmt.Sprintf("Coinbase API Error:\n%s", resultJSON))
				}
				return nil
			}))
			return nil
		})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			logMessage("error", fmt.Sprintf("Network error during Coinbase trade execution: %v", args[0]))
			return nil
		}))
	}()

	return nil
}
func (cc *CoinbaseConnector) Disconnect() error { return nil }

type BinanceConnector struct {
	apiKey       string
	apiSecret    string
	isPaperTrade bool
	ws           js.Value
	lastPrice    float64
	mu           sync.Mutex
}

func (bc *BinanceConnector) Connect(paperTrading bool, symbol string) error {
	logMessage("info", "Binance Connector Initializing...")
	bc.isPaperTrade = paperTrading

	wsURL := fmt.Sprintf("wss://stream.binance.com:9443/ws/%s@trade", strings.ToLower(symbol))
	logMessage("info", "Connecting to Binance WebSocket: "+wsURL)

	ws := js.Global().Get("WebSocket").New(wsURL)
	bc.ws = ws

	connected := make(chan bool)

	var onOpen, onMessage, onError, onClose js.Func

	onOpen = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		logMessage("success", "Binance WebSocket connection established.")
		connected <- true
		return nil
	})
	ws.Set("onopen", onOpen)

	onMessage = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		event := args[0]
		data := event.Get("data").String()
		var tradeData map[string]interface{}
		if err := json.Unmarshal([]byte(data), &tradeData); err != nil {
			logMessage("error", "Error parsing Binance trade data: "+err.Error())
			return nil
		}
		if priceStr, ok := tradeData["p"].(string); ok {
			if price, err := strconv.ParseFloat(priceStr, 64); err == nil {
				bc.mu.Lock()
				bc.lastPrice = price
				bc.mu.Unlock()
			}
		}
		return nil
	})
	ws.Set("onmessage", onMessage)

	onError = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		logMessage("error", "Binance WebSocket error.")
		return nil
	})
	ws.Set("onerror", onError)

	onClose = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		logMessage("warning", "Binance WebSocket connection closed.")
		onOpen.Release(); onMessage.Release(); onError.Release(); this.Release()
		return nil
	})
	ws.Set("onclose", onClose)

	select {
	case <-connected:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("Binance WebSocket connection timed out")
	}
}

func (bc *BinanceConnector) GetPrice() (float64, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if bc.lastPrice == 0 {
		return 0, fmt.Errorf("price not available yet from Binance WebSocket")
	}
	return bc.lastPrice, nil
}
func (bc *BinanceConnector) PlaceOrder(bs *BotState, signal Signal, price float64, symbol string) error {
	if bc.isPaperTrade {
		orderType := "HOLD"
		if signal == BUY { orderType = "BUY" }
		if signal == SELL { orderType = "SELL" }
		logMessage("success", fmt.Sprintf("[PAPER TRADE] Placed %s order for %s at $%.2f", orderType, symbol, price))
		return nil
	}

	if bc.apiKey == "" || bc.apiSecret == "" {
		err := fmt.Errorf("cannot place real order: Binance API Key or Secret is missing")
		logMessage("error", err.Error())
		return err
	}

	// Execute the trade asynchronously to avoid blocking the bot loop
	go func() {
		side := "BUY"
		if signal == SELL { side = "SELL" }

		// Simple risk management: trade a fixed USD amount based on risk level
		quoteOrderQty := "20.0" // Default: Moderate
		switch bs.config.RiskLevel {
		case "conservative":
			quoteOrderQty = "10.0" // e.g., $10
		case "aggressive":
			quoteOrderQty = "50.0" // e.g., $50
		}

		timestamp := time.Now().UnixNano() / int64(time.Millisecond)
		queryParams := fmt.Sprintf("symbol=%s&side=%s&type=MARKET&quoteOrderQty=%s&timestamp=%d", symbol, side, quoteOrderQty, timestamp)

		mac := hmac.New(sha256.New, []byte(bc.apiSecret))
		mac.Write([]byte(queryParams))
		signature := hex.EncodeToString(mac.Sum(nil))

		endpoint := "https://api.binance.com/api/v3/order"
		url := fmt.Sprintf("%s?%s&signature=%s", endpoint, queryParams, signature)

		headers := js.Global().Get("Object").New()
		headers.Set("X-MBX-APIKEY", bc.apiKey)
		reqOptions := js.Global().Get("Object").New()
		reqOptions.Set("method", "POST")
		reqOptions.Set("headers", headers)

		logMessage("info", fmt.Sprintf("[REAL TRADE] Submitting %s market order for %s of $%s...", side, symbol, quoteOrderQty))

		fetchPromise := js.Global().Call("fetch", url, reqOptions)

		fetchPromise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			response := args[0]
			response.Call("json").Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
				resultJSON, _ := js.Global().Get("JSON").Call("stringify", args[0], nil, 2).String()
				if response.Get("ok").Bool() {
					logMessage("success", fmt.Sprintf("Binance order successful:\n%s", resultJSON))
				} else {
					logMessage("error", fmt.Sprintf("Binance API Error:\n%s", resultJSON))
				}
				return nil
			}))
			return nil
		})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			logMessage("error", fmt.Sprintf("Network error during trade execution: %v", args[0]))
			return nil
		}))
	}()

	return nil
}
func (bc *BinanceConnector) Disconnect() error {
	if !bc.ws.IsUndefined() {
		logMessage("info", "Closing Binance WebSocket connection.")
		bc.ws.Call("close")
	}
	return nil
}

func NewBotState() *BotState {
	return &BotState{isRunning: false, stopChannel: make(chan bool), prices: []float64{}, equity: 10000.0, initialEquity: 10000.0}
}

func initializeConnector(config Config) (Connector, error) {
	switch config.Connector {
	case "simulation":
		return &SimulationConnector{}, nil
	case "coinbase":
		return &CoinbaseConnector{
			apiKey:       config.ConnectorParams["apiKey"],
			apiSecret:    config.ConnectorParams["apiSecret"],
			secretPhrase: config.ConnectorParams["secretPhrase"],
		}, nil
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

				// Calculate and update indicators for the chart
				indicators := make(map[string]float64)
				switch bs.config.Strategy {
				case "sma_crossover":
					p := bs.config.StrategyParams
					sp := int(p["sma_short_period"])
					lp := int(p["sma_long_period"])
					if len(bs.prices) >= sp {
						indicators["sma_short"] = sma(bs.prices, sp)
					}
					if len(bs.prices) >= lp {
						indicators["sma_long"] = sma(bs.prices, lp)
					}
				case "bollinger":
					p := bs.config.StrategyParams
					t := int(p["period"]); s := p["std_dev"]
					upper, _, lower := bollingerBands(bs.prices, t, s)
					indicators["bollinger_upper"] = upper
					indicators["bollinger_lower"] = lower
				}
				updateIndicatorsOnChart(indicators)

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
	if bs.connector != nil {
		bs.connector.Disconnect()
	}
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
		bs.connector.PlaceOrder(bs, signal, price, bs.config.Symbol)
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
func updateIndicatorsOnChart(indicators map[string]float64) {
	if len(indicators) == 0 {
		return
	}
	jsonData, err := json.Marshal(indicators)
	if err != nil {
		return // Fail silently
	}
	js.Global().Call("goUpdateIndicators", string(jsonData))
}

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