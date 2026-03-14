package tracker

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"scanner_bot/pkg/models"
)

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return &Store{db: db}, nil
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS trades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			signal_id TEXT UNIQUE NOT NULL,
			symbol TEXT NOT NULL,
			direction TEXT NOT NULL,
			pattern TEXT NOT NULL,
			grade TEXT NOT NULL,
			entry_price REAL NOT NULL,
			stop_loss REAL NOT NULL,
			tp1 REAL NOT NULL,
			tp2 REAL NOT NULL,
			tp3 REAL NOT NULL,
			exit_price REAL DEFAULT 0,
			status TEXT DEFAULT 'ACTIVE',
			pnl_percent REAL DEFAULT 0,
			current_price REAL DEFAULT 0,
			opened_at DATETIME NOT NULL,
			closed_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_trades_status ON trades(status);
		CREATE INDEX IF NOT EXISTS idx_trades_symbol ON trades(symbol);
	`)
	return err
}

func (s *Store) CreateTrade(sig *models.Signal) (*models.Trade, error) {
	entryPrice := (sig.EntryLow + sig.EntryHigh) / 2
	now := time.Now()

	result, err := s.db.Exec(`
		INSERT INTO trades (signal_id, symbol, direction, pattern, grade,
			entry_price, stop_loss, tp1, tp2, tp3, status, current_price, opened_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'ACTIVE', ?, ?)
	`, sig.ID, sig.Symbol, sig.Direction, sig.Pattern, sig.Grade,
		entryPrice, sig.StopLoss, sig.TP1, sig.TP2, sig.TP3, entryPrice, now)
	if err != nil {
		return nil, fmt.Errorf("insert trade: %w", err)
	}

	id, _ := result.LastInsertId()
	return &models.Trade{
		ID:         id,
		SignalID:   sig.ID,
		Symbol:     sig.Symbol,
		Direction:  sig.Direction,
		Pattern:    sig.Pattern,
		Grade:      sig.Grade,
		EntryPrice: entryPrice,
		StopLoss:   sig.StopLoss,
		TP1:        sig.TP1,
		TP2:        sig.TP2,
		TP3:        sig.TP3,
		Status:     models.TradeActive,
		OpenedAt:   now,
	}, nil
}

func (s *Store) UpdateTrade(id int64, status models.TradeStatus, exitPrice, pnl float64) error {
	now := time.Now()
	_, err := s.db.Exec(`
		UPDATE trades SET status = ?, exit_price = ?, pnl_percent = ?, closed_at = ?
		WHERE id = ?
	`, status, exitPrice, pnl, now, id)
	return err
}

func (s *Store) UpdateCurrentPrice(id int64, price float64) error {
	_, err := s.db.Exec(`UPDATE trades SET current_price = ? WHERE id = ?`, price, id)
	return err
}

func (s *Store) GetActiveTrades() ([]*models.Trade, error) {
	return s.queryTrades("WHERE status = 'ACTIVE'")
}

func (s *Store) GetAllTrades() ([]*models.Trade, error) {
	return s.queryTrades("ORDER BY opened_at DESC LIMIT 200")
}

func (s *Store) GetClosedTrades() ([]*models.Trade, error) {
	return s.queryTrades("WHERE status != 'ACTIVE' ORDER BY closed_at DESC LIMIT 200")
}

func (s *Store) queryTrades(where string) ([]*models.Trade, error) {
	rows, err := s.db.Query(`
		SELECT id, signal_id, symbol, direction, pattern, grade,
			entry_price, stop_loss, tp1, tp2, tp3,
			exit_price, status, pnl_percent, current_price,
			opened_at, closed_at
		FROM trades ` + where)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*models.Trade
	for rows.Next() {
		t := &models.Trade{}
		var closedAt sql.NullTime
		var dir, pattern, grade, status string

		err := rows.Scan(
			&t.ID, &t.SignalID, &t.Symbol, &dir, &pattern, &grade,
			&t.EntryPrice, &t.StopLoss, &t.TP1, &t.TP2, &t.TP3,
			&t.ExitPrice, &status, &t.PnLPercent, &t.CurrentPrice,
			&t.OpenedAt, &closedAt,
		)
		if err != nil {
			return nil, err
		}

		t.Direction = models.SignalDirection(dir)
		t.Pattern = models.PatternName(pattern)
		t.Grade = models.SignalGrade(grade)
		t.Status = models.TradeStatus(status)
		if closedAt.Valid {
			t.ClosedAt = &closedAt.Time
		}

		trades = append(trades, t)
	}

	return trades, rows.Err()
}

func (s *Store) GetStats() (*models.TradeStats, error) {
	trades, err := s.GetAllTrades()
	if err != nil {
		return nil, err
	}

	stats := &models.TradeStats{
		PatternStats: make(map[models.PatternName]*models.PatternStat),
	}

	for _, t := range trades {
		stats.TotalTrades++

		if t.Status == models.TradeActive {
			stats.ActiveTrades++
			continue
		}

		if t.PnLPercent > 0 {
			stats.WinTrades++
			stats.TotalPnL += t.PnLPercent
			if t.PnLPercent > stats.BestTrade {
				stats.BestTrade = t.PnLPercent
			}
		} else {
			stats.LossTrades++
			stats.TotalPnL += t.PnLPercent
			if t.PnLPercent < stats.WorstTrade {
				stats.WorstTrade = t.PnLPercent
			}
		}

		switch t.Status {
		case models.TradeTP1:
			stats.TP1Count++
		case models.TradeTP2:
			stats.TP2Count++
		case models.TradeTP3:
			stats.TP3Count++
		}

		// Pattern stats
		ps, ok := stats.PatternStats[t.Pattern]
		if !ok {
			ps = &models.PatternStat{Name: t.Pattern}
			stats.PatternStats[t.Pattern] = ps
		}
		ps.Total++
		if t.PnLPercent > 0 {
			ps.Wins++
		} else {
			ps.Losses++
		}
		ps.AvgPnL = (ps.AvgPnL*float64(ps.Total-1) + t.PnLPercent) / float64(ps.Total)
	}

	closed := stats.WinTrades + stats.LossTrades
	if closed > 0 {
		stats.WinRate = float64(stats.WinTrades) / float64(closed) * 100
		if stats.WinTrades > 0 {
			stats.AvgWin = stats.TotalPnL / float64(stats.WinTrades)
		}
		if stats.LossTrades > 0 {
			// avgLoss calculation using only losses
			totalLoss := 0.0
			for _, t := range trades {
				if t.PnLPercent < 0 {
					totalLoss += t.PnLPercent
				}
			}
			stats.AvgLoss = totalLoss / float64(stats.LossTrades)
		}
	}

	for _, ps := range stats.PatternStats {
		if ps.Total > 0 {
			ps.WinRate = float64(ps.Wins) / float64(ps.Total) * 100
		}
	}

	return stats, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
