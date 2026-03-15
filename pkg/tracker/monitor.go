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
	// TP3 reached -> close position
	if price >= trade.TP3 {
		m.closeTrade(trade, price, models.TradeTP3)
		return
	}

	// TP2 reached -> move stop to TP2 (never move backward)
	if price >= trade.TP2 {
		if trade.StopLoss < trade.TP2 {
			m.moveStopAndNotify(trade, trade.TP2, "TP2", price)
		}
	}

	// TP1 reached -> move stop to TP1 (only if not already at TP2)
	if price >= trade.TP1 {
		if trade.StopLoss < trade.TP1 {
			m.moveStopAndNotify(trade, trade.TP1, "TP1", price)
		}
	}

	// Stop hit -> close
	if price <= trade.StopLoss {
		m.closeTrade(trade, price, models.TradeStopped)
		return
	}
}

func (m *Monitor) evaluateShort(trade *models.Trade, price float64) {
	// TP3 reached -> close position
	if price <= trade.TP3 {
		m.closeTrade(trade, price, models.TradeTP3)
		return
	}

	// TP2 reached -> move stop to TP2 (never move backward)
	if price <= trade.TP2 {
		if trade.StopLoss > trade.TP2 {
			m.moveStopAndNotify(trade, trade.TP2, "TP2", price)
		}
	}

	// TP1 reached -> move stop to TP1 (only if not already at TP2)
	if price <= trade.TP1 {
		if trade.StopLoss > trade.TP1 {
			m.moveStopAndNotify(trade, trade.TP1, "TP1", price)
		}
	}

	// Stop hit -> close
	if price >= trade.StopLoss {
		m.closeTrade(trade, price, models.TradeStopped)
		return
	}
}

func (m *Monitor) moveStopAndNotify(trade *models.Trade, newStop float64, level string, currentPrice float64) {
	if err := m.store.UpdateStopLoss(trade.ID, newStop); err != nil {
		log.Printf("[Monitor] Error updating SL to %s for trade %d: %v", level, trade.ID, err)
		return
	}

	now := time.Now()
	if err := m.store.MarkStopMoved(trade.ID, level, now); err != nil {
		log.Printf("[Monitor] Error marking SL move timestamp (%s) for trade %d: %v", level, trade.ID, err)
	}

	trade.StopLoss = newStop
	trade.CurrentPrice = currentPrice
	if level == "TP1" {
		trade.MovedToTP1At = &now
	}
	if level == "TP2" {
		trade.MovedToTP2At = &now
	}
	log.Printf("[Monitor] %s %s trade #%d: SL moved to %s (%.8f)", trade.Symbol, trade.Direction, trade.ID, level, newStop)

	msg := signals.FormatStopMoved(trade, level, newStop)
	if err := m.tgBot.SendMessage(msg); err != nil {
		log.Printf("[Monitor] Error sending SL moved notification for trade %d: %v", trade.ID, err)
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
