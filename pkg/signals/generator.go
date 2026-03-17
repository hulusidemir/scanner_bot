package signals

import (
	"crypto/rand"
	"encoding/hex"
	"math"
	"time"

	"scanner_bot/pkg/analysis"
	"scanner_bot/pkg/models"
)

// Minimum profit targets (percentage from entry)
const (
	minTP1Percent  = 1.0   // Scalp TP1: fixed %1
	minTP2Percent  = 2.5   // TP2 minimum floor %2.5
	minTP3Percent  = 5.0   // TP3 minimum floor %5
	maxSLPercent   = 10.0  // Wide SL for testing: %10
	atrTP2Mult     = 3.5   // ATR multiplier for TP2
	atrTP3Mult     = 6.0   // ATR multiplier for TP3
	fallbackSLPct  = 10.0  // Fallback SL% when ATR is unavailable
)

// GenerateSignals creates trade signals from MTF analysis results
// STRICT: Only passes through the highest quality setups
func GenerateSignals(mtfResults []analysis.MTFResult) []*models.Signal {
	var signals []*models.Signal

	for _, r := range mtfResults {
		sig := buildSignal(r)
		if sig == nil {
			continue
		}

		// ══════════════════════════════════════════════════
		// QUALITY GATE 1: Minimum Grade A (score >= 70)
		// ══════════════════════════════════════════════════
		if sig.Grade != models.GradeAPlus && sig.Grade != models.GradeA {
			continue
		}

		// (R:R zorunluluğu kullanıcının isteği üzerine tamamen kaldırıldı)

		// ══════════════════════════════════════════════════
		// QUALITY GATE 3: Require orderbook support
		// ══════════════════════════════════════════════════
		if !r.HasOBSupport {
			continue
		}

		// ══════════════════════════════════════════════════
		// QUALITY GATE 4: Minimum 2 TF alignment
		// ══════════════════════════════════════════════════
		if r.AlignedTFs < 2 {
			continue
		}

		signals = append(signals, sig)
	}

	return signals
}

func buildSignal(r analysis.MTFResult) *models.Signal {
	// Get the primary TF metrics
	m, ok := r.Metrics[r.PrimaryTF]
	if !ok {
		return nil
	}

	price := m.LastPrice
	if price == 0 {
		return nil
	}

	// ── Grade thresholds ────────────────────────────
	grade := models.GradeB
	if r.ConfluenceScore >= 85 {
		grade = models.GradeAPlus
	} else if r.ConfluenceScore >= 75 {
		grade = models.GradeA
	}

	// Calculate entry zone, SL, TP based on direction
	var entryLow, entryHigh, sl, tp1, tp2, tp3 float64

	if r.Direction == models.DirectionLong {
		entryLow, entryHigh, sl, tp1, tp2, tp3 = calcLongLevels(price, m)
	} else {
		entryLow, entryHigh, sl, tp1, tp2, tp3 = calcShortLevels(price, m)
	}

	// Risk/Reward calculations
	var rrTP1, rrTP2, rrTP3 float64
	entryMid := (entryLow + entryHigh) / 2
	risk := math.Abs(entryMid - sl)
	if risk > 0 {
		rrTP1 = math.Abs(tp1-entryMid) / risk
		rrTP2 = math.Abs(tp2-entryMid) / risk
		rrTP3 = math.Abs(tp3-entryMid) / risk
	}

	// Determine L/S ratio from any available timeframe
	lsRatio := 0.0
	for _, mx := range r.Metrics {
		if mx.LSRatio > 0 {
			lsRatio = mx.LSRatio
			break
		}
	}

	return &models.Signal{
		ID:              generateID(),
		Symbol:          r.Symbol,
		Direction:       r.Direction,
		Pattern:         r.Pattern,
		Grade:           grade,
		Confidence:      r.ConfluenceScore,
		EntryLow:        entryLow,
		EntryHigh:       entryHigh,
		StopLoss:        sl,
		TP1:             tp1,
		TP2:             tp2,
		TP3:             tp3,
		RiskRewardTP1:   math.Round(rrTP1*10) / 10,
		RiskRewardTP2:   math.Round(rrTP2*10) / 10,
		RiskRewardTP3:   math.Round(rrTP3*10) / 10,
		Explanation:     r.Description,
		Metrics:         r.Metrics,
		LSRatio:         lsRatio,
		Volume24h:       m.Volume24h,
		FundingRate:     m.FundingRate,
		NextFundingTime: m.NextFundingTime,
		FundingInterval: m.FundingInterval,
		Timestamp:       time.Now(),
	}
}

// ════════════════════════════════════════════════════════════
// CORE FIX: TP/SL are now percentage-based with hard minimums
// No more 0.05% TPs — institutional-grade scalp levels
// ════════════════════════════════════════════════════════════

func calcLongLevels(price float64, m *models.TimeframeMetrics) (entryLow, entryHigh, sl, tp1, tp2, tp3 float64) {
	// Entry zone: tight range around current price
	if m.BidWallPrice > 0 && m.BidWallPrice < price && m.BidWallPrice > price*0.99 {
		entryLow = m.BidWallPrice
		entryHigh = price
	} else {
		entryLow = price * 0.998 // 0.2% below
		entryHigh = price * 1.001
	}

	entryMid := (entryLow + entryHigh) / 2

	// SL: fixed %10 below entry (wide SL for testing)
	sl = entryMid * (1 - maxSLPercent/100)

	// ── TP1: fixed %1 from entry ──────────────────────
	tp1 = entryMid * (1 + minTP1Percent/100)

	// ── TP2: ATR-based with minimum floor ─────────────
	if m.ATR > 0 {
		tp2 = entryMid + m.ATR*atrTP2Mult
	} else {
		tp2 = entryMid * (1 + minTP2Percent/100)
	}
	if tp2 < entryMid*(1+minTP2Percent/100) {
		tp2 = entryMid * (1 + minTP2Percent/100)
	}

	// ── TP3: ATR-based with minimum floor ─────────────
	if m.ATR > 0 {
		tp3 = entryMid + m.ATR*atrTP3Mult
	} else {
		tp3 = entryMid * (1 + minTP3Percent/100)
	}
	if tp3 < entryMid*(1+minTP3Percent/100) {
		tp3 = entryMid * (1 + minTP3Percent/100)
	}

	return
}

func calcShortLevels(price float64, m *models.TimeframeMetrics) (entryLow, entryHigh, sl, tp1, tp2, tp3 float64) {
	// Entry zone
	if m.AskWallPrice > 0 && m.AskWallPrice > price && m.AskWallPrice < price*1.01 {
		entryLow = price
		entryHigh = m.AskWallPrice
	} else {
		entryLow = price * 0.999
		entryHigh = price * 1.002 // 0.2% above
	}

	entryMid := (entryLow + entryHigh) / 2

	// SL: fixed %10 above entry (wide SL for testing)
	sl = entryMid * (1 + maxSLPercent/100)

	// ── TP1: fixed %1 from entry ──────────────────────
	tp1 = entryMid * (1 - minTP1Percent/100)

	// ── TP2: ATR-based with minimum floor ─────────────
	if m.ATR > 0 {
		tp2 = entryMid - m.ATR*atrTP2Mult
	} else {
		tp2 = entryMid * (1 - minTP2Percent/100)
	}
	if tp2 > entryMid*(1-minTP2Percent/100) {
		tp2 = entryMid * (1 - minTP2Percent/100)
	}

	// ── TP3: ATR-based with minimum floor ─────────────
	if m.ATR > 0 {
		tp3 = entryMid - m.ATR*atrTP3Mult
	} else {
		tp3 = entryMid * (1 - minTP3Percent/100)
	}
	if tp3 > entryMid*(1-minTP3Percent/100) {
		tp3 = entryMid * (1 - minTP3Percent/100)
	}

	return
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
