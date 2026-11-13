package ai

// USD per million tokens, Anthropic first-party rates as of Nov 2026.
// Unknown models cost 0 in the log rather than failing a call.
var pricing = map[string][2]float64{
	"claude-opus-5-5":   {4.00, 20.00},
	"claude-sonnet-5-5": {2.00, 10.00},
	"claude-haiku-5-5":  {0.10, 0.50},
}

// webSearchUSD is the per-search fee on top of tokens ($10 per 1,000).
const webSearchUSD = 0.01

func (c CallInfo) CostUSD() float64 {
	cost := float64(c.WebSearches) * webSearchUSD
	if p, ok := pricing[c.Model]; ok {
		cost += float64(c.InputTokens)*p[0]/1e6 + float64(c.OutputTokens)*p[1]/1e6
	}
	return cost
}
