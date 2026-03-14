package main

import (
	"log"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"scanner_bot/pkg/analysis"
	"scanner_bot/pkg/config"
	"scanner_bot/pkg/dashboard"
	"scanner_bot/pkg/exchange/binance"
	"scanner_bot/pkg/exchange/bybit"
	"scanner_bot/pkg/models"
	"scanner_bot/pkg/signals"
	"scanner_bot/pkg/telegram"
	"scanner_bot/pkg/tracker"
)

const version = "1.0.0"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("═══════════════════════════════════════")
	log.Println("  Scanner Bot v" + version)
	log.Println("  Perpetual Futures Signal Scanner")
	log.Println("═══════════════════════════════════════")

	// ── Load Config ────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}
	log.Printf("Config loaded: scan=%ds, minVol=$%.0f, port=%d",
		cfg.ScanIntervalSec, cfg.MinVolume24H, cfg.DashboardPort)

	// ── Initialize Clients ─────────────────────────────
	bybitClient := bybit.NewClient()
	binanceClient := binance.NewClient()
	tgBot := telegram.NewBot(cfg.TelegramBotToken, cfg.TelegramChatID)
	engine := analysis.NewEngine(bybitClient, binanceClient)

	// ── Initialize Trade Store ─────────────────────────
	store, err := tracker.NewStore("trades.db")
	if err != nil {
		log.Fatalf("Store error: %v", err)
	}
	defer store.Close()

	// ── Start Price Monitor ────────────────────────────
	monitor := tracker.NewMonitor(store, bybitClient, tgBot)
	monitor.Start()

	// ── Start Dashboard ────────────────────────────────
	dash := dashboard.NewServer(store, cfg.DashboardPort)
	dash.Start()

	// ── Initial Coin Fetch ─────────────────────────────
	coins, err := bybitClient.FetchInstruments()
	if err != nil {
		log.Fatalf("Failed to fetch instruments: %v", err)
	}
	log.Printf("Fetched %d USDT perpetual instruments", len(coins))

	// Send diagnostic
	tgBot.SendDiagnostic(len(coins), version)

	// ── Signal Tracking ────────────────────────────────
	// Keep track of recently sent signals to avoid duplicates
	recentSignals := make(map[string]time.Time)
	var recentMu sync.Mutex

	// ── Graceful Shutdown ──────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// ── Main Scan Loop ─────────────────────────────────
	scanTicker := time.NewTicker(time.Duration(cfg.ScanIntervalSec) * time.Second)
	defer scanTicker.Stop()

	// Do first scan immediately
	runScan(bybitClient, engine, tgBot, store, cfg, coins, recentSignals, &recentMu)

	for {
		select {
		case <-quit:
			log.Println("Shutting down gracefully...")
			monitor.Stop()
			tgBot.SendMessage("🔴 *Scanner Bot kapatıldı*")
			return
		case <-scanTicker.C:
			runScan(bybitClient, engine, tgBot, store, cfg, coins, recentSignals, &recentMu)
		}
	}
}

func runScan(
	bybitClient *bybit.Client,
	engine *analysis.Engine,
	tgBot *telegram.Bot,
	store *tracker.Store,
	cfg *config.Config,
	coins []models.Coin,
	recentSignals map[string]time.Time,
	recentMu *sync.Mutex,
) {
	start := time.Now()
	log.Printf("━━━ Scan started (%d coins) ━━━", len(coins))

	// Fetch current tickers for volume filter
	tickers, err := bybitClient.FetchTickers()
	if err != nil {
		log.Printf("Error fetching tickers: %v", err)
		return
	}

	// Filter coins by volume
	var filtered []*models.Coin
	for i := range coins {
		ticker, ok := tickers[coins[i].Symbol]
		if !ok {
			continue
		}
		coins[i].LastPrice = ticker.LastPrice
		coins[i].Turnover24h = ticker.Turnover24h
		coins[i].Volume24h = ticker.Volume24h
		coins[i].FundingRate = ticker.FundingRate
		coins[i].OpenInterest = ticker.OpenInterest
		coins[i].NextFundingTime = ticker.NextFundingTime
		coins[i].FundingInterval = ticker.FundingInterval

		if ticker.Turnover24h >= cfg.MinVolume24H {
			coin := coins[i] // copy
			filtered = append(filtered, &coin)
		}
	}

	// Sort by volume descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Turnover24h > filtered[j].Turnover24h
	})

	log.Printf("Volume filter: %d/%d coins passed ($%.0f+ threshold)",
		len(filtered), len(coins), cfg.MinVolume24H)

	// Semaphore for concurrent analysis (limit to 5 parallel)
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	var allSignals []*models.Signal
	var signalMu sync.Mutex

	for _, coin := range filtered {
		wg.Add(1)
		sem <- struct{}{}

		go func(c *models.Coin) {
			defer wg.Done()
			defer func() { <-sem }()

			// Analyze
			coinAnalysis := engine.AnalyzeCoin(c)
			if coinAnalysis == nil {
				return
			}

			// Multi-timeframe pattern matching
			mtfResults := analysis.AnalyzeMTF(coinAnalysis)
			if len(mtfResults) == 0 {
				return
			}

			// Generate signals
			sigs := signals.GenerateSignals(mtfResults)
			if len(sigs) == 0 {
				return
			}

			signalMu.Lock()
			allSignals = append(allSignals, sigs...)
			signalMu.Unlock()
		}(coin)
	}

	wg.Wait()

	// Process signals
	newSignals := 0
	recentMu.Lock()
	// Clean old entries (older than 2 hours)
	// Long cooldown prevents overtrading same coin
	for k, t := range recentSignals {
		if time.Since(t) > 2*time.Hour {
			delete(recentSignals, k)
		}
	}

	for _, sig := range allSignals {
		// Dedup key: symbol + direction (same coin same direction = skip)
		// Prevents multiple patterns triggering on the same setup
		key := string(sig.Symbol) + string(sig.Direction)
		if _, exists := recentSignals[key]; exists {
			continue
		}
		recentSignals[key] = time.Now()

		// Send to Telegram
		msg := signals.FormatTelegramMessage(sig)
		if err := tgBot.SendMessage(msg); err != nil {
			log.Printf("Error sending signal: %v", err)
			continue
		}

		// Record trade
		trade, err := store.CreateTrade(sig)
		if err != nil {
			log.Printf("Error creating trade: %v", err)
			continue
		}

		log.Printf("🔥 Signal: %s %s %s (Score: %d, Grade: %s, Trade #%d)",
			sig.Direction, sig.Symbol, sig.Pattern, sig.Confidence, sig.Grade, trade.ID)
		newSignals++
	}
	recentMu.Unlock()

	elapsed := time.Since(start)
	log.Printf("━━━ Scan complete: %d signals, %d new (%.1fs) ━━━",
		len(allSignals), newSignals, elapsed.Seconds())
}
