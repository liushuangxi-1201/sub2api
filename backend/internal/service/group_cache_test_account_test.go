//go:build unit

package service

import (
	"context"
	"fmt"
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

func TestGroupCacheBackgroundPrimesMatchingMessageBoundary(t *testing.T) {
	for _, mode := range []string{"responses", "responses OAuth", "chat"} {
		t.Run(mode, func(t *testing.T) {
			account := &Account{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
				Credentials: map[string]any{"api_key": "mock-key", "base_url": "http://cache.example/v1"},
				Extra:       map[string]any{openai_compat.ExtraKeyResponsesSupported: mode != "chat"},
			}
			if mode == "responses OAuth" {
				account.Type = AccountTypeOAuth
				account.Credentials = map[string]any{"access_token": "mock-oauth-token"}
			}
			stream := "data: " + `{"type":"response.output_text.delta","delta":"OK"}` + "\n\ndata: " +
				`{"type":"response.completed","response":{"usage":{"input_tokens":4000,"output_tokens":2,"input_tokens_details":{"cached_tokens":0,"cache_write_tokens":3900}}}}` + "\n\n"
			if mode == "chat" {
				stream = "data: " + `{"choices":[{"delta":{"content":"OK"},"finish_reason":"stop"}]}` + "\n\ndata: " +
					`{"usage":{"prompt_tokens":4000,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":0}}}` + "\n\ndata: [DONE]\n\n"
			}
			responses := make([]*http.Response, 3)
			for i := range responses {
				responses[i] = &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(stream))}
			}
			svc, upstream := adaptiveCNAccountTestService(account, responses...)
			// An administrator's reference may itself contain the reserved marker.
			prefix := "stable reference\n[cache-round]example in the reference\nReply with OK only."
			for round := 1; round <= 3; round++ {
				collector := &pelicanTestUsageCollector{}
				ctx := context.WithValue(context.Background(), pelicanTestUsageKey{}, collector)
				prompt := prefix + fmt.Sprintf("\n[cache-round]%d\nReply with OK only.", round)
				result, _, err := svc.RunCacheBackground(ctx, account.ID, "gpt-6-astra", prompt, "minimal", 256)
				require.NoError(t, err)
				require.Equal(t, "success", result.Status, result.ErrorMessage)
			}
			require.Len(t, upstream.bodies, 3)
			path, contentPath := "input", "content.0.text"
			if mode == "chat" {
				path, contentPath = "messages", "content"
			}
			baseline := gjson.GetBytes(upstream.bodies[0], path).Array()
			require.Len(t, baseline, 1, "the first request must write a boundary ending at the stable message")
			require.Equal(t, prefix, baseline[0].Get(contentPath).String())
			for i, body := range upstream.bodies[1:] {
				messages := gjson.GetBytes(body, path).Array()
				require.Len(t, messages, 2)
				require.Equal(t, baseline[0].Raw, messages[0].Raw, "a repeated request must retain the exact baseline message")
				require.Equal(t, fmt.Sprintf("[cache-round]%d\nReply with OK only.", i+2), messages[1].Get(contentPath).String())
			}
		})
	}
}

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
