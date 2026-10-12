package service

import (
	"context"
	"math"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// Collect only group-test requests. Each sample owns its collector, and each
// upstream stream owns a snapshot so cumulative usage is never added twice.
type pelicanTestUsageKey struct{}

type pelicanTestUsageCollector struct {
	model    string
	requests []*pelicanTestUsage
}

type pelicanTestUsage struct {
	protocol       string
	model          string
	serviceTier    string
	finishReason   string
	tokens         UsageTokens
	input          int
	output         int
	thoughts       int
	seen           bool
	inputSeen      bool
	cacheReadSeen  bool
	cacheWriteSeen bool
	outputSeen     bool
	complete       bool
	invalid        bool
}

func startPelicanTestStream(c *gin.Context, protocol string) *pelicanTestUsage {
	if c.Request == nil {
		return nil
	}
	return startPelicanTestUsage(c.Request.Context(), protocol)
}

func recordPelicanTestSSE(ctx context.Context, protocol, model string, body []byte) {
	u := startPelicanTestUsage(ctx, protocol)
	if u == nil {
		return
	}
	u.model = model
	for _, line := range strings.Split(string(body), "\n") {
		if data, ok := strings.CutPrefix(strings.TrimSpace(line), "data:"); ok {
			u.read(strings.TrimSpace(data))
		}
	}
}

func pelicanUsageFromContext(ctx context.Context) *pelicanTestUsageCollector {
	u, _ := ctx.Value(pelicanTestUsageKey{}).(*pelicanTestUsageCollector)
	return u
}

func startPelicanTestUsage(ctx context.Context, protocol string) *pelicanTestUsage {
	collector := pelicanUsageFromContext(ctx)
	if collector == nil {
		return nil
	}
	u := &pelicanTestUsage{protocol: protocol, model: collector.model}
	collector.requests = append(collector.requests, u)
	return u
}

func (u *pelicanTestUsage) read(raw string) {
	if u == nil {
		return
	}
	data := gjson.Parse(raw)
	eventType := data.Get("type").String()
	if reason := data.Get("delta.stop_reason").String(); reason != "" {
		u.finishReason = reason
	}
	if eventType == "message_stop" || eventType == "response.completed" || eventType == "response.done" || eventType == "response.failed" || eventType == "response.incomplete" {
		u.complete = true
	}
	if response := data.Get("response"); response.IsObject() {
		data = response
	} else if message := data.Get("message"); message.IsObject() {
		data = message
	}
	if model := strings.TrimSpace(data.Get("model").String()); model != "" {
		u.model = model
	}
	if tier := data.Get("service_tier").String(); tier != "" {
		u.serviceTier = tier
	}
	usage := data.Get("usage")
	if u.protocol == "gemini" {
		usage = data.Get("usageMetadata")
		for _, candidate := range data.Get("candidates").Array() {
			if candidate.Get("finishReason").String() != "" {
				u.complete = true
			}
		}
	}
	for _, choice := range data.Get("choices").Array() {
		if reason := choice.Get("finish_reason").String(); reason != "" {
			u.finishReason = reason
			u.complete = true
		}
	}
	if !usage.IsObject() {
		return
	}
	set := func(field string, value *int) {
		n := usage.Get(field)
		if !n.Exists() {
			return
		}
		if n.Type != gjson.Number || n.Float() < 0 || n.Float() > 1_000_000_000 || math.IsNaN(n.Float()) || math.IsInf(n.Float(), 0) || n.Float() != math.Trunc(n.Float()) {
			u.invalid = true
			return
		}
		*value = int(n.Int())
		u.seen = true
		if value == &u.input {
			u.inputSeen = true
		}
		if value == &u.tokens.CacheReadTokens {
			u.cacheReadSeen = true
		}
		if value == &u.tokens.CacheCreationTokens {
			u.cacheWriteSeen = true
		}
		if value == &u.output {
			u.outputSeen = true
		}
	}
	switch u.protocol {
	case "anthropic":
		set("input_tokens", &u.input)
		set("output_tokens", &u.output)
		set("cache_read_input_tokens", &u.tokens.CacheReadTokens)
		set("cache_creation_input_tokens", &u.tokens.CacheCreationTokens)
		set("cache_creation.ephemeral_5m_input_tokens", &u.tokens.CacheCreation5mTokens)
		set("cache_creation.ephemeral_1h_input_tokens", &u.tokens.CacheCreation1hTokens)
	case "gemini":
		set("promptTokenCount", &u.input)
		set("candidatesTokenCount", &u.output)
		set("thoughtsTokenCount", &u.thoughts)
		set("cachedContentTokenCount", &u.tokens.CacheReadTokens)
	case "chat":
		set("prompt_tokens", &u.input)
		set("completion_tokens", &u.output)
		set("prompt_tokens_details.cached_tokens", &u.tokens.CacheReadTokens)
		set("prompt_cache_hit_tokens", &u.tokens.CacheReadTokens)
	default:
		set("input_tokens", &u.input)
		set("output_tokens", &u.output)
		set("input_tokens_details.cached_tokens", &u.tokens.CacheReadTokens)
		set("cache_read_input_tokens", &u.tokens.CacheReadTokens)
		set("cache_creation_input_tokens", &u.tokens.CacheCreationTokens)
		set("input_tokens_details.cache_write_tokens", &u.tokens.CacheCreationTokens)
	}
	u.tokens.InputTokens = u.input
	if u.protocol != "anthropic" {
		// OpenAI/Chat/Gemini input totals already include cache hits.
		if u.inputSeen && u.tokens.CacheReadTokens+u.tokens.CacheCreationTokens > u.input {
			u.invalid = true
		}
		u.tokens.InputTokens = max(0, u.input-u.tokens.CacheReadTokens-u.tokens.CacheCreationTokens)
	}
	u.tokens.OutputTokens = u.output + u.thoughts
}

// Return a cost snapshot, not a debit. Use the account's upstream cost multiplier,
// never the customer/group selling multiplier. Missing usage/pricing remains
// unknown; a partially metered attempt must not discard other known costs.
func (c *pelicanTestUsageCollector) cost(billing *BillingService, account *Account) (*float64, bool) {
	var total float64
	priced, incomplete := false, len(c.requests) == 0
	for _, u := range c.requests {
		if !u.seen || u.invalid || billing == nil {
			incomplete = true
			continue
		}
		cost, err := billing.CalculateCostWithServiceTier(u.model, u.tokens, 1, u.serviceTier)
		if err != nil || cost == nil {
			incomplete = true
			continue
		}
		amount := cost.TotalCost * account.CostMultiplier()
		if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
			incomplete = true
			continue
		}
		total += amount
		priced = true
		incomplete = incomplete || !u.complete || !u.inputSeen || !u.outputSeen
	}
	if !priced {
		return nil, true
	}
	return &total, incomplete
}
