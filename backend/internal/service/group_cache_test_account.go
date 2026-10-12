package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// An account experiment reuses the proven Pelican transport but asks for a short
// text answer. No HTML is generated, and probes never debit a customer balance.
type cacheTestTimingKey struct{}
type cacheTestTiming struct {
	started time.Time
	first   *int
}

func (s *AccountTestService) RunCacheBackground(ctx context.Context, accountID int64, model, prompt, effort string, maxOutput int) (*ScheduledTestResult, *int, error) {
	started := time.Now()
	timing := &cacheTestTiming{started: started}
	ctx = context.WithValue(ctx, cacheTestTimingKey{}, timing)
	ctx = withPelicanTestOptions(ctx, pelicanTestOptions{prompt: prompt, reasoningEffort: effort, observeOnly: true, cacheProbe: true, maxOutput: maxOutput})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	w := &pelicanRecorder{ResponseRecorder: httptest.NewRecorder(), cancel: cancel}
	c, _ := gin.CreateTestContext(w)
	c.Request = (&http.Request{Header: make(http.Header)}).WithContext(ctx)
	err := s.TestPelicanAccountConnection(c, accountID, model, prompt, effort)
	output, message := parsePelicanOutput(w.Body.String())
	if err != nil && message == "" {
		message = "Cache request failed; inspect server test logs"
	}
	if w.overflow {
		message = pelicanErrCaptureLimit
	}
	if message == "" && strings.TrimSpace(output) == "" {
		message = pelicanErrEmptyOutput
	}
	// A test_complete synthesized after EOF is not enough: upstream metering must
	// also report its terminal event. This catches truncated Claude responses.
	collector := pelicanUsageFromContext(ctx)
	if message == "" && (collector == nil || len(collector.requests) == 0 || !collector.requests[len(collector.requests)-1].complete) {
		message = "Generation stream ended before completion"
	}
	if message == "" && collector != nil && len(collector.requests) > 0 {
		switch collector.requests[len(collector.requests)-1].finishReason {
		case "length", "max_tokens":
			message = pelicanErrMaxTokens
		case "content_filter", "refusal":
			message = pelicanErrRefused
		}
	}
	status := "success"
	if message != "" {
		status = "failed"
	}
	return &ScheduledTestResult{Status: status, ResponseText: output, ErrorMessage: message, StartedAt: started, FinishedAt: time.Now(), LatencyMs: time.Since(started).Milliseconds()}, timing.first, nil
}

// Apply only to cache probes; existing Pelican and connection tests retain
// their payloads. Claude caches the shared prefix, excluding the round suffix.
func applyCacheProbePayload(ctx context.Context, payload map[string]any, protocol string, oauth bool) {
	options, ok := pelicanTestOptionsFromContext(ctx)
	if !ok || !options.cacheProbe {
		return
	}
	switch protocol {
	case "messages":
		payload["max_tokens"] = options.maxOutput
		prefix, suffix, found := strings.Cut(options.prompt, "\n[cache-round]")
		if found {
			payload["messages"] = []map[string]any{{"role": "user", "content": []map[string]any{
				{"type": "text", "text": prefix, "cache_control": map[string]any{"type": "ephemeral"}},
				{"type": "text", "text": "[cache-round]" + suffix},
			}}}
		}
	case "chat":
		payload["max_completion_tokens"] = options.maxOutput
	case "responses":
		if !oauth {
			payload["max_output_tokens"] = options.maxOutput
		}
	}
}
