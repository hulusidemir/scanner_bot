package tracker

import (
	"log"
	"time"

	"scanner_bot/pkg/exchange/bybit"
	"scanner_bot/pkg/models"
	"scanner_bot/pkg/signals"
	"scanner_bot/pkg/telegram"
)

type Monitor struct {
	store    *Store
	bybit    *bybit.Client
	tgBot    *telegram.Bot
	interval time.Duration
	stopCh   chan struct{}
}

func NewMonitor(store *Store, bybitClient *bybit.Client, tgBot *telegram.Bot) *Monitor {
	return &Monitor{
		store:    store,
		bybit:    bybitClient,
		tgBot:    tgBot,
		interval: 5 * time.Second,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the background price monitoring loop
func (m *Monitor) Start() {
	go m.loop()
	log.Println("[Monitor] Price monitoring started (interval: 5s)")
}

func (m *Monitor) Stop() {
	close(m.stopCh)
}

func (m *Monitor) loop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkActiveTrades()
		}
	}
}

func (m *Monitor) checkActiveTrades() {
	trades, err := m.store.GetActiveTrades()
	if err != nil {
		log.Printf("[Monitor] Error getting active trades: %v", err)
		return
	}

	if len(trades) == 0 {
		return
	}

	// Fetch current prices
	tickers, err := m.bybit.FetchTickers()
	if err != nil {
		log.Printf("[Monitor] Error fetching tickers: %v", err)
		return
	}

	for _, trade := range trades {
		ticker, ok := tickers[trade.Symbol]
		if !ok {
			continue
		}

		currentPrice := ticker.LastPrice
		if currentPrice == 0 {
			continue
		}

		// Update current price in DB
		m.store.UpdateCurrentPrice(trade.ID, currentPrice)

		// Check TP/SL conditions based on direction
		m.evaluateTrade(trade, currentPrice)
	}
}

func (m *Monitor) evaluateTrade(trade *models.Trade, currentPrice float64) {
	if trade.Direction == models.DirectionLong {
		m.evaluateLong(trade, currentPrice)
	} else {
		m.evaluateShort(trade, currentPrice)
	}
}

func (m *Monitor) evaluateLong(trade *models.Trade, price float64) {
	// Check SL first
	if price <= trade.StopLoss {
		m.closeTrade(trade, price, models.TradeStopped)
		return
	}

	// Check TPs in reverse order (TP3 → TP2 → TP1)
	if price >= trade.TP3 {
		m.closeTrade(trade, price, models.TradeTP3)
		return
	}
	if price >= trade.TP2 {
		// Don't close yet — wait for TP3 or SL
		// But track that TP2 was reached
		return
	}
	if price >= trade.TP1 {
		// TP1 reached. For simplicity, we close at the highest TP reached
		// Check if price is retreating back towards entry
		// We use a trailing approach: if price was above TP1 but drops below entry mid, close at TP1
		midEntry := trade.EntryPrice
		if price < midEntry*1.002 {
			// Price retreating after TP1 — close at TP1
			m.closeTrade(trade, trade.TP1, models.TradeTP1)
		}
		// Otherwise keep open for TP2/TP3
	}
}

func (m *Monitor) evaluateShort(trade *models.Trade, price float64) {
	// Check SL first
	if price >= trade.StopLoss {
		m.closeTrade(trade, price, models.TradeStopped)
		return
	}

	// Check TPs (for short, TP prices are below entry)
	if price <= trade.TP3 {
		m.closeTrade(trade, price, models.TradeTP3)
		return
	}
	if price <= trade.TP2 {
		return
	}
	if price <= trade.TP1 {
		midEntry := trade.EntryPrice
		if price > midEntry*0.998 {
			m.closeTrade(trade, trade.TP1, models.TradeTP1)
		}
	}
}

func (m *Monitor) closeTrade(trade *models.Trade, exitPrice float64, status models.TradeStatus) {
	// Calculate PnL
	var pnl float64
	if trade.Direction == models.DirectionLong {
		pnl = ((exitPrice - trade.EntryPrice) / trade.EntryPrice) * 100
	} else {
		pnl = ((trade.EntryPrice - exitPrice) / trade.EntryPrice) * 100
	}

	err := m.store.UpdateTrade(trade.ID, status, exitPrice, pnl)
	if err != nil {
		log.Printf("[Monitor] Error closing trade %d: %v", trade.ID, err)
		return
	}

	trade.ExitPrice = exitPrice
	trade.Status = status
	trade.PnLPercent = pnl

	log.Printf("[Monitor] Trade %s %s closed: %s PnL: %.2f%%",
		trade.Symbol, trade.Direction, status, pnl)

	// Send Telegram notification
	msg := signals.FormatTradeClose(trade)
	if err := m.tgBot.SendMessage(msg); err != nil {
		log.Printf("[Monitor] Error sending close notification: %v", err)
	}
}
