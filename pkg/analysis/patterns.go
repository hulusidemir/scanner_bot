package analysis

import (
	"scanner_bot/pkg/models"
)

// PatternDef defines a single pattern's matching criteria
type PatternDef struct {
	Name        models.PatternName
	Direction   models.SignalDirection
	Description string
	Match       func(m *models.TimeframeMetrics) bool
}

// AllPatterns are the 22+ named pattern definitions
var AllPatterns = []PatternDef{
	{
		Name:      models.PatternStealthAccumulation,
		Direction: models.DirectionShort,
		Description: "OI artarken spot'ta agresif alım var ama perp tarafında satış baskısı devam ediyor. " +
			"Bu pattern, akıllı paranın spot'ta biriktirirken perp'te hedge yaptığını gösterir. " +
			"Bid wall güçlü destek oluşturuyor.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend >= models.TrendUp &&
				m.PerpCVDTrend <= models.TrendDown &&
				m.SpotCVDTrend >= models.TrendUp &&
				m.OBBias >= models.OBBidWall
		},
	},
	{
		Name:      models.PatternAggressiveDistro,
		Direction: models.DirectionLong,
		Description: "Perp tarafında güçlü alım var ama spot'ta satış baskısı mevcut. " +
			"OI artışı yeni pozisyon açıldığını gösteriyor ancak spot CVD negatif. " +
			"Klasik tuzak kurulumu — perp pump + spot dump.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend >= models.TrendUp &&
				m.PerpCVDTrend >= models.TrendUp &&
				m.SpotCVDTrend <= models.TrendDown &&
				m.OBBias <= models.OBAskWall
		},
	},
	{
		Name:      models.PatternWhaleSqueezeSetup,
		Direction: models.DirectionShort,
		Description: "Aşırı short pozisyon açılmış (OI çok yüksek), perp CVD negatif, " +
			"ama ask tarafı ince. Bir squeeze tetiklendiğinde fiyat hızla yükselebilir.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend >= models.TrendStrongUp &&
				m.PerpCVDTrend <= models.TrendStrongDown &&
				m.SpotCVDTrend == models.TrendNeutral &&
				m.OBBias <= models.OBAskWall // thin ask = easy to push up
		},
	},
	{
		Name:      models.PatternCapitulationReversal,
		Direction: models.DirectionShort,
		Description: "Pozisyonlar hızla kapanıyor (OI düşüyor) ama spot tarafında alım var. " +
			"Bid wall güçlü destek oluşturuyor. Kapitülasyon sonrası dönüş sinyali.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend <= models.TrendStrongDown &&
				m.PerpCVDTrend <= models.TrendDown &&
				m.SpotCVDTrend >= models.TrendUp &&
				m.OBBias >= models.OBBidWall
		},
	},
	{
		Name:      models.PatternSmartMoneyShort,
		Direction: models.DirectionLong,
		Description: "OI artıyor yani yeni pozisyon açılıyor. Perp CVD nötr ama spot'ta " +
			"agresif satış var. Akıllı para spot'ta satıyor ve short pozisyon alıyor.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend >= models.TrendUp &&
				m.PerpCVDTrend == models.TrendNeutral &&
				m.SpotCVDTrend <= models.TrendStrongDown &&
				m.OBBias <= models.OBAskWall
		},
	},
	{
		Name:      models.PatternRetailFOMOTrap,
		Direction: models.DirectionLong,
		Description: "Aşırı long pozisyon açılmış (OI ve perp CVD çok yüksek). " +
			"Bid desteği zayıf. Retail FOMO ile pompalanan fiyat geri çekilebilir.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend >= models.TrendStrongUp &&
				m.PerpCVDTrend >= models.TrendStrongUp &&
				m.SpotCVDTrend == models.TrendNeutral &&
				m.OBBias <= models.OBAskWall // thin bid support
		},
	},
	{
		Name:      models.PatternSilentDistribution,
		Direction: models.DirectionLong,
		Description: "OI sabit, perp alım var ama spot'ta sessiz satış devam ediyor. " +
			"Büyük oyuncular spot'ta dağıtım yaparken perp'te fiyatı tutuyor.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend == models.TrendNeutral &&
				m.PerpCVDTrend >= models.TrendUp &&
				m.SpotCVDTrend <= models.TrendDown &&
				m.OBBias <= models.OBAskWall
		},
	},
	{
		Name:      models.PatternDivergentStrength,
		Direction: models.DirectionShort,
		Description: "Tüm metrikler aynı yönde: OI artıyor, hem perp hem spot CVD pozitif, " +
			"orderbook bid ağırlıklı. Güçlü trend devam sinyali.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend >= models.TrendUp &&
				m.PerpCVDTrend >= models.TrendUp &&
				m.SpotCVDTrend >= models.TrendUp &&
				m.OBBias >= models.OBBidWall
		},
	},
	{
		Name:      models.PatternBearishConvergence,
		Direction: models.DirectionLong,
		Description: "OI artıyor (yeni pozisyon) ama hem perp hem spot CVD negatif. " +
			"Ask tarafı baskın. Tüm akış verileri aşağı yönlü — güçlü düşüş sinyali.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend >= models.TrendUp &&
				m.PerpCVDTrend <= models.TrendDown &&
				m.SpotCVDTrend <= models.TrendDown &&
				m.OBBias <= models.OBAskWall
		},
	},
	{
		Name:      models.PatternFundingDivergence,
		Direction: models.DirectionShort,
		Description: "Funding rate negatif (short'lar ödüyor), OI artıyor, spot alım var. " +
			"Piyasa aşırı short ama akıllı para spot'ta biriktiriyor. Reversal yakın.",
		Match: func(m *models.TimeframeMetrics) bool {
			// TIGHTENED: Requires strong OI + spot CVD + bid support
			return m.OITrend >= models.TrendUp &&
				m.PerpCVDTrend <= models.TrendDown &&
				m.SpotCVDTrend >= models.TrendUp &&
				m.OBBias >= models.OBBidWall &&
				m.FundingRate < -0.0005 // requires meaningful negative funding
		},
	},
	{
		Name:      models.PatternLiqCascadeShort,
		Direction: models.DirectionLong,
		Description: "OI hızla düşüyor, perp CVD çok negatif, bid desteği ince. " +
			"Tasfiye kaskadı devam ediyor — momentum aşağı yönlü.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend <= models.TrendStrongDown &&
				m.PerpCVDTrend <= models.TrendStrongDown &&
				m.SpotCVDTrend <= models.TrendDown &&
				m.OBBias <= models.OBAskWall
		},
	},
	{
		Name:      models.PatternLiqCascadeLong,
		Direction: models.DirectionShort,
		Description: "OI düşüyor (short pozisyonlar tasfiye), perp CVD çok pozitif, " +
			"spot alım var. Short squeeze tetiklendi — yukarı momentum güçlü.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend <= models.TrendStrongDown &&
				m.PerpCVDTrend >= models.TrendStrongUp &&
				m.SpotCVDTrend >= models.TrendUp &&
				m.OBBias <= models.OBAskWall // thin ask = easy to break
		},
	},
	{
		Name:      models.PatternAbsorption,
		Direction: models.DirectionShort,
		Description: "OI sabit, perp CVD pozitif, kalın bid wall var. " +
			"Büyük alıcı satış baskısını emiyor — fiyat kırılım için biriktiyor.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend == models.TrendNeutral &&
				m.PerpCVDTrend >= models.TrendUp &&
				m.SpotCVDTrend == models.TrendNeutral &&
				m.OBBias >= models.OBBidHeavy
		},
	},
	{
		Name:      models.PatternHiddenSelling,
		Direction: models.DirectionLong,
		Description: "OI sabit, perp CVD negatif, kalın ask wall var. " +
			"Büyük satıcı alım baskısını emiyor — fiyat aşağı kırılım yapabilir.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend == models.TrendNeutral &&
				m.PerpCVDTrend <= models.TrendDown &&
				m.SpotCVDTrend == models.TrendNeutral &&
				m.OBBias <= models.OBAskHeavy
		},
	},
	{
		Name:      models.PatternExhaustionTop,
		Direction: models.DirectionLong,
		Description: "OI düşerken perp CVD pozitif — son nefes pompası. " +
			"Pozisyon kapanırken fiyatı yükseltenler var ama spot satış devam ediyor. Tepe tükenme sinyali.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend <= models.TrendDown &&
				m.PerpCVDTrend >= models.TrendUp &&
				m.SpotCVDTrend <= models.TrendDown
		},
	},
	{
		Name:      models.PatternExhaustionBottom,
		Direction: models.DirectionShort,
		Description: "OI düşerken spot CVD pozitif. Pozisyonlar kapanıyor ama spot alım devam ediyor. " +
			"Dip toplama sinyali — akıllı para ucuzdan alıyor.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OITrend <= models.TrendDown &&
				m.PerpCVDTrend <= models.TrendDown &&
				m.SpotCVDTrend >= models.TrendUp &&
				m.OBBias >= models.OBBidWall
		},
	},
	// ── Multi-Timeframe & Extended Patterns ──────────
	{
		Name:      models.PatternFundingExtremeLong,
		Direction: models.DirectionShort,
		Description: "Funding rate aşırı negatif (short'lar çok ödüyor). " +
			"Bu seviye tarihsel olarak reversal noktası. Short squeeze olasılığı yüksek.",
		Match: func(m *models.TimeframeMetrics) bool {
			// TIGHTENED: Requires very negative funding + OI rising + spot buying + bid support
			return m.FundingRate < -0.001 &&
				m.OITrend >= models.TrendUp &&
				m.SpotCVDTrend >= models.TrendUp &&
				m.OBBias >= models.OBBidWall
		},
	},
	{
		Name:      models.PatternFundingExtremeShort,
		Direction: models.DirectionLong,
		Description: "Funding rate aşırı pozitif (long'lar çok ödüyor). " +
			"Aşırı kalabalık long pozisyon. Long squeeze olasılığı yüksek.",
		Match: func(m *models.TimeframeMetrics) bool {
			// TIGHTENED: Requires very positive funding + OI rising + spot selling + ask pressure
			return m.FundingRate > 0.001 &&
				m.OITrend >= models.TrendUp &&
				m.SpotCVDTrend <= models.TrendDown &&
				m.OBBias <= models.OBAskWall
		},
	},
	{
		Name:      models.PatternOBImbalanceBull,
		Direction: models.DirectionShort,
		Description: "Orderbook çok güçlü bid ağırlıklı. Büyük alıcılar defansif pozisyonda. " +
			"OI artışı ile beraber güçlü destek oluşuyor.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OBImbalance > 2.0 &&
				m.OITrend >= models.TrendUp &&
				m.PerpCVDTrend >= models.TrendNeutral
		},
	},
	{
		Name:      models.PatternOBImbalanceBear,
		Direction: models.DirectionLong,
		Description: "Orderbook çok güçlü ask ağırlıklı. Büyük satıcılar agresif pozisyonda. " +
			"OI artışı ile beraber güçlü direnç oluşuyor.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.OBImbalance < 0.5 &&
				m.OITrend >= models.TrendUp &&
				m.PerpCVDTrend <= models.TrendNeutral
		},
	},
	// ── Pure Bybit Patterns (No Binance CVD Required) ──────────
	// These patterns allow coins listed only on Bybit to trigger signals
	// by compensating for the lack of CVD with stronger OI and OB requirements.
	{
		Name:      "Pure Bybit Momentum Long",
		Direction: models.DirectionShort,
		Description: "Sadece Bybit verisi: Güçlü Open Interest artışı ve devasa Bid duvarı. " +
			"CVD verisi olmayan (sadece Bybit'te olan) coinler için momentum kurulumu.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.SpotCVD == 0 && m.PerpCVD == 0 && // Only trigger if Binance data is missing
				m.OITrend >= models.TrendStrongUp &&
				m.OBBias >= models.OBBidHeavy
		},
	},
	{
		Name:      "Pure Bybit Momentum Short",
		Direction: models.DirectionLong,
		Description: "Sadece Bybit verisi: Güçlü Open Interest artışı ve devasa Ask duvarı. " +
			"CVD verisi olmayan (sadece Bybit'te olan) coinler için momentum kurulumu.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.SpotCVD == 0 && m.PerpCVD == 0 && // Only trigger if Binance data is missing
				m.OITrend >= models.TrendStrongUp &&
				m.OBBias <= models.OBAskHeavy
		},
	},
	{
		Name:      "Pure Bybit Funding Exhaustion",
		Direction: models.DirectionShort,
		Description: "Sadece Bybit verisi: Devasa negatif funding, fiyat düşüşü ama Bid duvarı tutuyor. " +
			"CVD verisi olmayan coinlerde short sıkıştırma (squeeze) kurulumu.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.SpotCVD == 0 && m.PerpCVD == 0 && // Only trigger if Binance data is missing
				m.FundingRate < -0.001 &&
				m.OITrend >= models.TrendUp &&
				m.OBBias >= models.OBBidWall
		},
	},
	{
		Name:      "Pure Bybit Funding Exhaustion Short",
		Direction: models.DirectionLong,
		Description: "Sadece Bybit verisi: Devasa pozitif funding, fiyat artışı ama Ask duvarı tutuyor. " +
			"CVD verisi olmayan coinlerde long sıkıştırma (squeeze) kurulumu.",
		Match: func(m *models.TimeframeMetrics) bool {
			return m.SpotCVD == 0 && m.PerpCVD == 0 && // Only trigger if Binance data is missing
				m.FundingRate > 0.001 &&
				m.OITrend >= models.TrendUp &&
				m.OBBias <= models.OBAskWall
		},
	},
}

// ClassifyPatterns finds all matching patterns for the given metrics
func ClassifyPatterns(m *models.TimeframeMetrics) []PatternMatch {
	var matches []PatternMatch
	for _, p := range AllPatterns {
		if p.Match(m) {
			matches = append(matches, PatternMatch{
				Pattern:     p.Name,
				Direction:   p.Direction,
				Description: p.Description,
			})
		}
	}
	return matches
}

type PatternMatch struct {
	Pattern     models.PatternName
	Direction   models.SignalDirection
	Description string
}
