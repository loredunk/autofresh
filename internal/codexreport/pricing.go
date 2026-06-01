package codexreport

import "strings"

// price holds per-million-token rates in USD. cachedInput is the discounted rate
// for cache hits; output already includes reasoning tokens (Codex bills
// reasoning_output_tokens within output_tokens).
type price struct {
	input       float64
	cachedInput float64
	output      float64
}

// priceTable maps a normalized model family prefix to its rates. These are
// ESTIMATES for reporting only — Codex rollouts do not record billed cost, so we
// approximate from published list prices. Matching is longest-prefix so that
// more specific keys (e.g. a "-codex" variant) win over the family default.
//
// All figures are USD per 1,000,000 tokens. Rates are standard OpenAI API
// text-token prices, last checked against official pricing on 2026-06-01.
var priceTable = []struct {
	prefix string
	p      price
}{
	{"gpt-5.5", price{input: 5.0, cachedInput: 0.5, output: 30.0}},
	{"gpt-5.4-mini", price{input: 0.75, cachedInput: 0.075, output: 4.5}},
	{"gpt-5.4-nano", price{input: 0.20, cachedInput: 0.02, output: 1.25}},
	{"gpt-5.4", price{input: 2.50, cachedInput: 0.25, output: 15.0}},
	{"gpt-5.3-codex", price{input: 1.75, cachedInput: 0.175, output: 14.0}},
	{"gpt-5.2-codex", price{input: 1.75, cachedInput: 0.175, output: 14.0}},
	{"gpt-5.2", price{input: 1.75, cachedInput: 0.175, output: 14.0}},
	{"gpt-5.1", price{input: 1.25, cachedInput: 0.125, output: 10.0}},
	{"gpt-5-mini", price{input: 0.25, cachedInput: 0.025, output: 2.0}},
	{"gpt-5", price{input: 1.25, cachedInput: 0.125, output: 10.0}},
}

// normalizeModel canonicalizes model strings like "gpt5.4" -> "gpt-5.4" so the
// price table matches consistently.
func normalizeModel(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	if strings.HasPrefix(m, "gpt5") && !strings.HasPrefix(m, "gpt-5") {
		m = "gpt-5" + strings.TrimPrefix(m, "gpt5")
	}
	return m
}

// lookupPrice returns the rates for a model and whether a match was found.
func lookupPrice(model string) (price, bool) {
	m := normalizeModel(model)
	bestLen := -1
	var best price
	for _, e := range priceTable {
		if strings.HasPrefix(m, e.prefix) && len(e.prefix) > bestLen {
			bestLen = len(e.prefix)
			best = e.p
		}
	}
	return best, bestLen >= 0
}

// estimateCost computes the USD estimate for a token increment under a model.
// Cached input is billed at the discounted rate; the remaining (non-cached)
// input at the full rate. Returns whether the model was priced.
func estimateCost(d tokenUsage, model string) (float64, bool) {
	p, ok := lookupPrice(model)
	if !ok {
		return 0, false
	}
	nonCached := d.Input - d.CachedInput
	if nonCached < 0 {
		nonCached = 0
	}
	cost := float64(nonCached)*p.input/1e6 +
		float64(d.CachedInput)*p.cachedInput/1e6 +
		float64(d.Output)*p.output/1e6
	return cost, true
}
