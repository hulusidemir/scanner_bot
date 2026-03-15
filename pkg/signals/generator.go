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
	minTP1Percent  = 0.4  // User requested 0.4% minimum for TP1
	minTP2Percent  = 0.0  // No minimum enforced for TP2
	minTP3Percent  = 0.0  // No minimum enforced for TP3
	maxSLPercent   = 3.0  // Maximum 3.0% stop loss from entry (cap)
	atrMultiplier  = 2.5  // ATR multiplier for stop loss
	fallbackSLPct  = 1.5  // Fallback SL% when ATR is unavailable
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
	} else if r.ConfluenceScore >= 70 {
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

	// SL: ATR × 2.5 below entry
	if m.ATR > 0 {
		sl = entryMid - m.ATR*atrMultiplier
	} else {
		sl = entryMid * (1 - fallbackSLPct/100)
	}

	// Enforce maximum SL distance (cap at maxSLPercent)
	maxSL := entryMid * (1 - maxSLPercent/100)
	if sl < maxSL {
		sl = maxSL
	}

	// Risk distance (always positive for long)
	risk := entryMid - sl

	// TP levels: use R:R multiples but enforce MINIMUMS
	rawTP1 := entryMid + risk*1.5
	rawTP2 := entryMid + risk*2.5
	rawTP3 := entryMid + risk*4.0

	// Enforce minimum profit percentages
	minTP1Price := entryMid * (1 + minTP1Percent/100)
	minTP2Price := entryMid * (1 + minTP2Percent/100)
	minTP3Price := entryMid * (1 + minTP3Percent/100)

	tp1 = math.Max(rawTP1, minTP1Price)
	tp2 = math.Max(rawTP2, minTP2Price)
	tp3 = math.Max(rawTP3, minTP3Price)

	// If ask wall exists and is below TP1, push TP1 just past it
	if m.AskWallPrice > 0 && m.AskWallPrice > price && m.AskWallPrice < tp1 {
		// Only use wall if it's above our minimum
		if m.AskWallPrice*0.999 >= minTP1Price {
			tp1 = m.AskWallPrice * 0.999
		}
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

	// SL: ATR × 2.5 above entry
	if m.ATR > 0 {
		sl = entryMid + m.ATR*atrMultiplier
	} else {
		sl = entryMid * (1 + fallbackSLPct/100)
	}

	// Enforce maximum SL distance (cap at maxSLPercent)
	maxSL := entryMid * (1 + maxSLPercent/100)
	if sl > maxSL {
		sl = maxSL
	}

	// Risk distance
	risk := sl - entryMid

	// TP levels with multipliers
	rawTP1 := entryMid - risk*1.5
	rawTP2 := entryMid - risk*2.5
	rawTP3 := entryMid - risk*4.0

	// Enforce minimum profit percentages (for short, price goes DOWN)
	minTP1Price := entryMid * (1 - minTP1Percent/100)
	minTP2Price := entryMid * (1 - minTP2Percent/100)
	minTP3Price := entryMid * (1 - minTP3Percent/100)

	tp1 = math.Min(rawTP1, minTP1Price)
	tp2 = math.Min(rawTP2, minTP2Price)
	tp3 = math.Min(rawTP3, minTP3Price)

	// If bid wall exists and above TP1, use as TP1 target
	if m.BidWallPrice > 0 && m.BidWallPrice < price && m.BidWallPrice > tp1 {
		if m.BidWallPrice*1.001 <= minTP1Price {
			tp1 = m.BidWallPrice * 1.001
		}
	}

	return
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
