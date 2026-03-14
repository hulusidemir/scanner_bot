package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

type Bot struct {
	token    string
	chatID   string
	http     *http.Client
	limiter  chan struct{}
	mu       sync.Mutex
}

func NewBot(token, chatID string) *Bot {
	b := &Bot{
		token:   token,
		chatID:  chatID,
		http:    &http.Client{Timeout: 10 * time.Second},
		limiter: make(chan struct{}, 1),
	}
	b.limiter <- struct{}{}
	go func() {
		ticker := time.NewTicker(time.Second / 20) // 20 msg/sec max
		defer ticker.Stop()
		for range ticker.C {
			select {
			case b.limiter <- struct{}{}:
			default:
			}
		}
	}()
	return b
}

func (b *Bot) SendMessage(text string) error {
	<-b.limiter

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", b.token)

	payload := map[string]interface{}{
		"chat_id":    b.chatID,
		"text":       text,
		"parse_mode": "Markdown",
		"disable_web_page_preview": true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	resp, err := b.http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("send failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (b *Bot) SendDiagnostic(coinCount int, version string) {
	msg := fmt.Sprintf(
		"🤖 *Scanner Bot Başlatıldı*\n\n"+
			"📊 Taranan coin sayısı: %d\n"+
			"⏰ Tarama aralığı: 5 dakika\n"+
			"📈 Timeframe'ler: 5m, 15m, 1h, 4h\n"+
			"🔍 Pattern sayısı: 20+\n"+
			"📡 Veri kaynakları: Bybit + Binance\n"+
			"🌐 Dashboard: http://localhost:8081\n"+
			"🔄 Versiyon: %s\n\n"+
			"✅ Sistem hazır, tarama başlıyor...",
		coinCount, version,
	)

	if err := b.SendMessage(msg); err != nil {
		log.Printf("Failed to send diagnostic: %v", err)
	}
}
