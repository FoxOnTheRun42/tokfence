package budget

import "strings"

type ModelPricing struct {
	InputPerMTok  float64
	OutputPerMTok float64
}

var Pricing = map[string]ModelPricing{
	"claude-sonnet-4-5-20250514": {InputPerMTok: 3.00, OutputPerMTok: 15.00},
	"claude-haiku-4-5-20250514":  {InputPerMTok: 0.80, OutputPerMTok: 4.00},
	"claude-opus-4-5-20250514":   {InputPerMTok: 15.00, OutputPerMTok: 75.00},

	"gpt-4o":      {InputPerMTok: 2.50, OutputPerMTok: 10.00},
	"gpt-4o-mini": {InputPerMTok: 0.15, OutputPerMTok: 0.60},
	"o1":          {InputPerMTok: 15.00, OutputPerMTok: 60.00},
	"o3":          {InputPerMTok: 10.00, OutputPerMTok: 40.00},
	"gpt-5":       {InputPerMTok: 5.00, OutputPerMTok: 20.00},
}

func EstimateCostCents(model string, inputTokens, outputTokens int) int64 {
	model = strings.TrimSpace(model)
	pricing, ok := Pricing[model]
	if !ok {
		return 0
	}
	inputCostUSD := (float64(inputTokens) / 1_000_000.0) * pricing.InputPerMTok
	outputCostUSD := (float64(outputTokens) / 1_000_000.0) * pricing.OutputPerMTok
	usd := inputCostUSD + outputCostUSD
	return int64(usd * 100.0)
}
