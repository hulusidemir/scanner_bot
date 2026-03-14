# Scanner Bot 🤖

Institutional-grade algorithmic trading signal generator for Bybit Perpetual Futures.

This bot continuously scans the entire Bybit Perpetual Futures market (coins with >$10M daily volume) to identify high-probability institutional setups based on Multi-Timeframe (MTF) analysis, Open Interest, Orderbook Imbalances, and CVD (Cumulative Volume Delta) metrics.

## Features
- **Real-Time Market Scanning**: Monitors 5m, 15m, 1h, and 4h timeframes simultaneously.
- **Institutional Market Regimes**: Identifies stealth accumulation, aggressive distribution, whale squeezes, and capitulation reversals.
- **Pure Bybit Setups**: Fully supports massive volume meme coins listed only on Bybit.
- **Telegram Integration**: Sends beautifully formatted, actionable signals with TP/SL levels directly to your Telegram.
- **Performance Dashboard**: Real-time web panel tracking the performance of generated signals with a gorgeous dark-mode UI.

## Requirements
- Go 1.21+
- A Telegram Bot Token & Chat ID

## Deployment Guide (Oracle Free Tier / Ubuntu)

1. **Clone the repository:**
   ```bash
   git clone <your-repo-url>
   cd scanner_bot
   ```

2. **Configure your environment:**
   ```bash
   cp .env.example .env
   # Edit the .env file with your Telegram credentials
   nano .env
   ```
   *Make sure `DASHBOARD_PORT=8081`

3. **Run
   go run ./cmd/scanner/
   # Press Ctrl+A then D to detach and leave it running in the background.
   ```

## Disclaimer
This software is for educational purposes only. Cryptocurrency trading carries a high level of risk. The generated signals are based on algorithmic probabilities and do not guarantee profits.
