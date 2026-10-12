//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGroupCacheBackgroundRejectsTruncatedOrRefusedStreams(t *testing.T) {
	for _, tc := range []struct {
		name, protocol, stream string
		success                bool
	}{
		{"chat completes", "chat", `{"choices":[{"delta":{"content":"OK"},"finish_reason":"stop"}]}` + "\n\ndata: " + `{"usage":{"prompt_tokens":1000,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":900}}}` + "\n\ndata: [DONE]", true},
		{"chat output is cut off", "chat", `{"choices":[{"delta":{"content":"partial"},"finish_reason":"length"}]}` + "\n\ndata: " + `{"usage":{"prompt_tokens":1000,"completion_tokens":256,"prompt_tokens_details":{"cached_tokens":900}}}` + "\n\ndata: [DONE]", false},
		{"chat refuses", "chat", `{"choices":[{"delta":{"content":"partial"},"finish_reason":"content_filter"}]}` + "\n\ndata: [DONE]", false},
		{"claude premature EOF", "messages", `{"type":"message_start","message":{"usage":{"input_tokens":1000,"output_tokens":1,"cache_read_input_tokens":900}}}` + "\n\ndata: " + `{"type":"content_block_delta","delta":{"type":"text_delta","text":"OK"}}`, false},
		{"claude output is cut off", "messages", `{"type":"message_start","message":{"usage":{"input_tokens":1000,"output_tokens":1,"cache_read_input_tokens":900}}}` + "\n\ndata: " + `{"type":"content_block_delta","delta":{"type":"text_delta","text":"partial"}}` + "\n\ndata: " + `{"type":"message_delta","delta":{"stop_reason":"max_tokens"},"usage":{"output_tokens":256}}` + "\n\ndata: " + `{"type":"message_stop"}`, false},
		{"claude completes", "messages", `{"type":"message_start","message":{"usage":{"input_tokens":1000,"output_tokens":1,"cache_read_input_tokens":900}}}` + "\n\ndata: " + `{"type":"content_block_delta","delta":{"type":"text_delta","text":"OK"}}` + "\n\ndata: " + `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}` + "\n\ndata: " + `{"type":"message_stop"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &Account{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
				Credentials: map[string]any{"api_key": "mock-key", "base_url": "http://cache.example/v1"},
				Extra:       map[string]any{openai_compat.ExtraKeyResponsesSupported: false},
			}
			model := "demo-model"
			if tc.protocol == "messages" {
				account.Platform = PlatformAnthropic
				model = "claude-sonnet-4-5"
			}
			upstream := &httpUpstreamRecorder{responses: []*http.Response{{StatusCode: http.StatusOK,
				Header: http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:   io.NopCloser(strings.NewReader("data: " + tc.stream + "\n\n")),
			}}}
			svc := pelicanClaudeTestService(account, upstream)
			svc.cfg = rawChatCompletionsTestConfig()
			collector := &pelicanTestUsageCollector{model: model}
			ctx := context.WithValue(context.Background(), pelicanTestUsageKey{}, collector)
			result, first, err := svc.RunCacheBackground(ctx, account.ID, model, "stable reference\n[cache-round]2\nReply with OK only.", "minimal", 256)
			require.NoError(t, err)
			require.NotNil(t, first, "the stream includes text, even in failed generations")
			require.Len(t, upstream.requests, 1, "stream failures do not replay the generation")
			if tc.success {
				require.Equal(t, "success", result.Status, result.ErrorMessage)
				require.Equal(t, "OK", result.ResponseText)
			} else {
				require.Equal(t, "failed", result.Status)
				require.NotEmpty(t, result.ErrorMessage)
			}
			body := upstream.lastBody
			if tc.protocol == "messages" {
				require.EqualValues(t, 256, gjson.GetBytes(body, "max_tokens").Int())
			} else {
				require.EqualValues(t, 256, gjson.GetBytes(body, "max_completion_tokens").Int())
			}
		})
	}
}

func TestGroupCacheFirstTextTimingIgnoresReasoningAndBlankContent(t *testing.T) {
	svc := &AccountTestService{}
	timing := &cacheTestTiming{started: time.Now().Add(-time.Second)}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/test", nil).WithContext(context.WithValue(context.Background(), cacheTestTimingKey{}, timing))
	svc.sendEvent(c, TestEvent{Type: "thinking", Text: "reasoning"})
	svc.sendEvent(c, TestEvent{Type: "content", Text: " \n"})
	require.Nil(t, timing.first)
	svc.sendEvent(c, TestEvent{Type: "content", Text: "OK"})
	require.NotNil(t, timing.first)
	first := *timing.first
	require.GreaterOrEqual(t, first, 1000)
	svc.sendEvent(c, TestEvent{Type: "content", Text: "later"})
	require.Equal(t, first, *timing.first)
}
