package sandbox

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/noyitz/tools/bbr-sandbox/internal/cluster"
	"github.com/noyitz/tools/bbr-sandbox/internal/extproc"
	"github.com/noyitz/tools/bbr-sandbox/internal/presets"
)

// Handlers holds the API handler state.
type Handlers struct {
	mu        sync.RWMutex
	client    *extproc.Client
	namespace string
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(namespace string) *Handlers {
	return &Handlers{namespace: namespace}
}

type connectRequest struct {
	Target string `json:"target"`
}

type connectResponse struct {
	Connected bool   `json:"connected"`
	Target    string `json:"target"`
	Error     string `json:"error,omitempty"`
}

type sendRequest struct {
	Headers map[string]string `json:"headers"`
	Body    map[string]any    `json:"body"`
}

// HandleConnect connects to a BBR ext-proc server.
func (h *Handlers) HandleConnect(w http.ResponseWriter, r *http.Request) {
	var req connectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, connectResponse{Error: "invalid request body"})
		return
	}

	if req.Target == "" {
		writeJSON(w, http.StatusBadRequest, connectResponse{Error: "target is required"})
		return
	}

	client, err := extproc.NewClient(req.Target)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, connectResponse{Error: err.Error()})
		return
	}

	// Test connectivity
	if err := client.Ping(r.Context()); err != nil {
		client.Close()
		writeJSON(w, http.StatusBadGateway, connectResponse{Error: err.Error()})
		return
	}

	// Replace existing client
	h.mu.Lock()
	if h.client != nil {
		h.client.Close()
	}
	h.client = client
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, connectResponse{Connected: true, Target: req.Target})
}

// HandleHealth returns the current connection status.
func (h *Handlers) HandleHealth(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	resp := connectResponse{}
	if h.client != nil {
		resp.Connected = true
		resp.Target = h.client.Target()
		if err := h.client.Ping(r.Context()); err != nil {
			resp.Connected = false
			resp.Error = err.Error()
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleSendRequest sends a request through the BBR ext-proc flow.
func (h *Handlers) HandleSendRequest(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	client := h.client
	h.mu.RUnlock()

	if client == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "not connected to BBR server"})
		return
	}

	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	result, err := client.SendRequest(r.Context(), req.Headers, req.Body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// HandleSendResponse sends a response through the BBR ext-proc flow.
func (h *Handlers) HandleSendResponse(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	client := h.client
	h.mu.RUnlock()

	if client == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "not connected to BBR server"})
		return
	}

	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	result, err := client.SendResponse(r.Context(), req.Headers, req.Body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// HandlePresets returns all available sample request/response templates.
func (h *Handlers) HandlePresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, presets.All())
}

// HandleClusterInfo returns cluster and BBR deployment info.
func (h *Handlers) HandleClusterInfo(w http.ResponseWriter, r *http.Request) {
	info := cluster.GetInfo(r.Context(), h.namespace)
	writeJSON(w, http.StatusOK, info)
}

type clusterLoginRequest struct {
	Server    string `json:"server"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Namespace string `json:"namespace"`
}

// HandleClusterLogin logs into an OpenShift cluster.
func (h *Handlers) HandleClusterLogin(w http.ResponseWriter, r *http.Request) {
	var req clusterLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Server == "" || req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "server, username, and password are required"})
		return
	}

	if req.Namespace != "" {
		h.mu.Lock()
		h.namespace = req.Namespace
		h.mu.Unlock()
	}

	if err := cluster.Login(r.Context(), req.Server, req.Username, req.Password); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	// Return fresh cluster info
	h.mu.RLock()
	ns := h.namespace
	h.mu.RUnlock()
	info := cluster.GetInfo(r.Context(), ns)
	writeJSON(w, http.StatusOK, info)
}

// HandleClusterLogout logs out of the current cluster and disconnects the gRPC client.
func (h *Handlers) HandleClusterLogout(w http.ResponseWriter, r *http.Request) {
	// Disconnect gRPC client
	h.mu.Lock()
	if h.client != nil {
		h.client.Close()
		h.client = nil
	}
	h.mu.Unlock()

	_ = cluster.Logout(r.Context())
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
