package extproc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corepb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocpb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a gRPC ext-proc client that simulates Envoy's communication with BBR.
type Client struct {
	conn   *grpc.ClientConn
	client extprocpb.ExternalProcessorClient
	target string
}

// HeaderMutations holds the set and removed headers from a ProcessingResponse.
type HeaderMutations struct {
	Set     map[string]string `json:"set"`
	Removed []string          `json:"removed"`
}

// BodyMutation holds the mutated body from a ProcessingResponse.
type BodyMutation struct {
	Body    map[string]any `json:"body,omitempty"`
	Raw     string         `json:"raw,omitempty"`
	Cleared bool           `json:"cleared,omitempty"`
}

// ExtProcResult holds the complete result of an ext-proc exchange.
type ExtProcResult struct {
	Phase                   string          `json:"phase"`
	HeaderMutations         HeaderMutations `json:"headerMutations"`
	BodyMutation            *BodyMutation   `json:"bodyMutation,omitempty"`
	ClearRouteCache         bool            `json:"clearRouteCache"`
	FinalHeaders            map[string]string `json:"finalHeaders"`
	FinalBody               map[string]any  `json:"finalBody,omitempty"`
	Error                   string          `json:"error,omitempty"`
	DurationMs              float64         `json:"durationMs"`
}

// NewClient creates a new ext-proc gRPC client.
func NewClient(target string) (*Client, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", target, err)
	}
	return &Client{
		conn:   conn,
		client: extprocpb.NewExternalProcessorClient(conn),
		target: target,
	}, nil
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Target returns the current target address.
func (c *Client) Target() string {
	return c.target
}

// Ping checks if the BBR server is reachable by opening and immediately closing a stream.
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	stream, err := c.client.Process(ctx)
	if err != nil {
		return fmt.Errorf("cannot reach BBR at %s: %w", c.target, err)
	}
	_ = stream.CloseSend()
	return nil
}

// SendRequest simulates Envoy sending a request through the ext-proc flow.
// It sends RequestHeaders followed by RequestBody, and collects the mutations.
func (c *Client) SendRequest(ctx context.Context, headers map[string]string, body map[string]any) (*ExtProcResult, error) {
	start := time.Now()

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	stream, err := c.client.Process(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open ext-proc stream: %w", err)
	}
	defer stream.CloseSend()

	// Send RequestHeaders
	headerMap := buildHeaderMap(headers)
	err = stream.Send(&extprocpb.ProcessingRequest{
		Request: &extprocpb.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocpb.HttpHeaders{
				Headers:     headerMap,
				EndOfStream: false,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send request headers: %w", err)
	}

	// Receive headers response
	headersResp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive headers response: %w", err)
	}

	// Send RequestBody
	err = stream.Send(&extprocpb.ProcessingRequest{
		Request: &extprocpb.ProcessingRequest_RequestBody{
			RequestBody: &extprocpb.HttpBody{
				Body:        bodyBytes,
				EndOfStream: true,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send request body: %w", err)
	}

	// Receive body response
	bodyResp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive body response: %w", err)
	}

	result := buildResult("request", headers, body, headersResp, bodyResp)
	result.DurationMs = float64(time.Since(start).Microseconds()) / 1000.0
	return result, nil
}

// SendResponse simulates Envoy sending a response through the ext-proc flow.
func (c *Client) SendResponse(ctx context.Context, headers map[string]string, body map[string]any) (*ExtProcResult, error) {
	start := time.Now()

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response body: %w", err)
	}

	stream, err := c.client.Process(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open ext-proc stream: %w", err)
	}
	defer stream.CloseSend()

	// For response processing, we first need to send empty request headers/body
	// to get past the request phase, then send response headers/body.
	// Send minimal request headers (end of stream)
	err = stream.Send(&extprocpb.ProcessingRequest{
		Request: &extprocpb.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocpb.HttpHeaders{
				Headers:     buildHeaderMap(map[string]string{"content-type": "application/json"}),
				EndOfStream: true,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send pass-through request headers: %w", err)
	}

	// Receive request headers response
	_, err = stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive request headers response: %w", err)
	}

	// Send ResponseHeaders
	headerMap := buildHeaderMap(headers)
	err = stream.Send(&extprocpb.ProcessingRequest{
		Request: &extprocpb.ProcessingRequest_ResponseHeaders{
			ResponseHeaders: &extprocpb.HttpHeaders{
				Headers:     headerMap,
				EndOfStream: false,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send response headers: %w", err)
	}

	// Receive response headers response
	headersResp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response headers response: %w", err)
	}

	// Send ResponseBody
	err = stream.Send(&extprocpb.ProcessingRequest{
		Request: &extprocpb.ProcessingRequest_ResponseBody{
			ResponseBody: &extprocpb.HttpBody{
				Body:        bodyBytes,
				EndOfStream: true,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send response body: %w", err)
	}

	// Receive response body response
	bodyResp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response body response: %w", err)
	}

	result := buildResult("response", headers, body, headersResp, bodyResp)
	result.DurationMs = float64(time.Since(start).Microseconds()) / 1000.0
	return result, nil
}

func buildHeaderMap(headers map[string]string) *corepb.HeaderMap {
	hm := &corepb.HeaderMap{}
	for k, v := range headers {
		hm.Headers = append(hm.Headers, &corepb.HeaderValue{
			Key:   k,
			Value: v,
		})
	}
	return hm
}

func buildResult(phase string, originalHeaders map[string]string, originalBody map[string]any, headersResp, bodyResp *extprocpb.ProcessingResponse) *ExtProcResult {
	result := &ExtProcResult{
		Phase: phase,
		HeaderMutations: HeaderMutations{
			Set:     map[string]string{},
			Removed: []string{},
		},
	}

	// Extract mutations from both header and body responses
	collectMutations(headersResp, result)
	collectMutations(bodyResp, result)

	// Build final state
	result.FinalHeaders = make(map[string]string)
	for k, v := range originalHeaders {
		result.FinalHeaders[k] = v
	}
	for k, v := range result.HeaderMutations.Set {
		result.FinalHeaders[k] = v
	}
	for _, k := range result.HeaderMutations.Removed {
		delete(result.FinalHeaders, k)
	}

	// Final body
	if result.BodyMutation != nil && result.BodyMutation.Body != nil {
		result.FinalBody = result.BodyMutation.Body
	} else {
		result.FinalBody = originalBody
	}

	return result
}

func collectMutations(resp *extprocpb.ProcessingResponse, result *ExtProcResult) {
	if resp == nil {
		return
	}

	var common *extprocpb.CommonResponse

	switch r := resp.Response.(type) {
	case *extprocpb.ProcessingResponse_RequestHeaders:
		if r.RequestHeaders != nil {
			common = r.RequestHeaders.Response
		}
	case *extprocpb.ProcessingResponse_RequestBody:
		if r.RequestBody != nil {
			common = r.RequestBody.Response
		}
	case *extprocpb.ProcessingResponse_ResponseHeaders:
		if r.ResponseHeaders != nil {
			common = r.ResponseHeaders.Response
		}
	case *extprocpb.ProcessingResponse_ResponseBody:
		if r.ResponseBody != nil {
			common = r.ResponseBody.Response
		}
	}

	if common == nil {
		return
	}

	if common.ClearRouteCache {
		result.ClearRouteCache = true
	}

	// Header mutations
	if common.HeaderMutation != nil {
		for _, h := range common.HeaderMutation.SetHeaders {
			if h.Header != nil {
				// BBR uses RawValue (bytes) instead of Value (string)
				val := h.Header.Value
				if len(h.Header.RawValue) > 0 {
					val = string(h.Header.RawValue)
				}
				result.HeaderMutations.Set[h.Header.Key] = val
			}
		}
		result.HeaderMutations.Removed = append(result.HeaderMutations.Removed, common.HeaderMutation.RemoveHeaders...)
	}

	// Body mutation
	if common.BodyMutation != nil {
		switch m := common.BodyMutation.Mutation.(type) {
		case *extprocpb.BodyMutation_Body:
			var parsed map[string]any
			if err := json.Unmarshal(m.Body, &parsed); err == nil {
				result.BodyMutation = &BodyMutation{Body: parsed}
			} else {
				result.BodyMutation = &BodyMutation{Raw: string(m.Body)}
			}
		case *extprocpb.BodyMutation_ClearBody:
			if m.ClearBody {
				result.BodyMutation = &BodyMutation{Cleared: true}
			}
		}
	}
}
