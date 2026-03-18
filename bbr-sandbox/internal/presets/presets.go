package presets

// Preset defines a sample request or response template.
type Preset struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Phase       string            `json:"phase"` // "request" or "response"
	Headers     map[string]string `json:"headers"`
	Body        map[string]any    `json:"body"`
}

// All returns all available presets.
func All() []Preset {
	return []Preset{
		{
			ID:          "openai-chat-basic",
			Name:        "OpenAI Chat",
			Description: "Basic OpenAI chat completion request (passthrough, no translation)",
			Phase:       "request",
			Headers: map[string]string{
				"content-type":  "application/json",
				"authorization": "Bearer sk-test-key-12345",
			},
			Body: map[string]any{
				"model": "gpt-4o",
				"messages": []any{
					map[string]any{"role": "user", "content": "What is 2+2?"},
				},
				"max_tokens": float64(100),
			},
		},
		{
			ID:          "anthropic-translation",
			Name:        "Anthropic Translation",
			Description: "OpenAI request routed to Anthropic provider (triggers full API translation)",
			Phase:       "request",
			Headers: map[string]string{
				"content-type":                  "application/json",
				"authorization":                 "Bearer sk-test-key-12345",
				"X-Gateway-Destination-Provider": "anthropic",
			},
			Body: map[string]any{
				"model": "claude-sonnet-4-20250514",
				"messages": []any{
					map[string]any{"role": "user", "content": "Hello, how are you?"},
				},
				"max_tokens": float64(256),
			},
		},
		{
			ID:          "anthropic-with-system",
			Name:        "System Messages",
			Description: "Request with system and developer messages (extracted to Anthropic system param)",
			Phase:       "request",
			Headers: map[string]string{
				"content-type":                  "application/json",
				"authorization":                 "Bearer sk-test-key-12345",
				"X-Gateway-Destination-Provider": "anthropic",
			},
			Body: map[string]any{
				"model": "claude-sonnet-4-20250514",
				"messages": []any{
					map[string]any{"role": "system", "content": "You are a helpful assistant."},
					map[string]any{"role": "developer", "content": "Be concise in your responses."},
					map[string]any{"role": "user", "content": "Explain quantum computing."},
				},
				"max_tokens":            float64(512),
				"max_completion_tokens": float64(200),
				"temperature":           0.7,
			},
		},
		{
			ID:          "anthropic-with-tools",
			Name:        "Tool Use",
			Description: "Request with tool definitions for testing tool_use translation",
			Phase:       "request",
			Headers: map[string]string{
				"content-type":                  "application/json",
				"authorization":                 "Bearer sk-test-key-12345",
				"X-Gateway-Destination-Provider": "anthropic",
			},
			Body: map[string]any{
				"model": "claude-sonnet-4-20250514",
				"messages": []any{
					map[string]any{"role": "user", "content": "What's the weather in San Francisco?"},
				},
				"max_tokens": float64(256),
			},
		},
		{
			ID:          "anthropic-response-success",
			Name:        "Anthropic Response",
			Description: "Anthropic message response (translated back to OpenAI format)",
			Phase:       "response",
			Headers: map[string]string{
				"content-type": "application/json",
			},
			Body: map[string]any{
				"id":    "msg_01XYZ",
				"type":  "message",
				"role":  "assistant",
				"model": "claude-sonnet-4-20250514",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": "Hello! I'm doing well, thank you for asking.",
					},
				},
				"stop_reason": "end_turn",
				"usage": map[string]any{
					"input_tokens":  float64(12),
					"output_tokens": float64(15),
				},
			},
		},
		{
			ID:          "anthropic-response-error",
			Name:        "Anthropic Error",
			Description: "Anthropic error response (translated to OpenAI error format)",
			Phase:       "response",
			Headers: map[string]string{
				"content-type": "application/json",
			},
			Body: map[string]any{
				"type": "error",
				"error": map[string]any{
					"type":    "invalid_request_error",
					"message": "max_tokens: 999999 > 4096, which is the maximum allowed",
				},
			},
		},
		{
			ID:          "openai-response",
			Name:        "OpenAI Response",
			Description: "OpenAI chat completion response (passthrough, no mutation expected)",
			Phase:       "response",
			Headers: map[string]string{
				"content-type": "application/json",
			},
			Body: map[string]any{
				"id":      "chatcmpl-abc123",
				"object":  "chat.completion",
				"created": float64(1710000000),
				"model":   "gpt-4o",
				"choices": []any{
					map[string]any{
						"index": float64(0),
						"message": map[string]any{
							"role":    "assistant",
							"content": "4",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]any{
					"prompt_tokens":     float64(10),
					"completion_tokens": float64(1),
					"total_tokens":      float64(11),
				},
			},
		},
	}
}
