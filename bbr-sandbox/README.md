# BBR Plugin Sandbox

An interactive developer tool for testing BBR (Body-Based Router) plugins. It simulates Envoy's gRPC ext-proc protocol against a real BBR pod running in a Kubernetes/OpenShift cluster, letting you craft requests, send them through the plugin chain, and visualize exactly what the plugins changed.

## How It Works

```
Your Machine                                          Cluster
+---------------------------+                         +---------------------------+
|                           |                         |  bbr-plugins namespace    |
|  Browser --- HTTP ---> Go Backend --- gRPC --->  Port Forward  ------>  BBR Pod  ---> Plugin Chain
|  (Web UI)              (Envoy Simulator)         (oc port-forward)     (:9004)       (body-field-to-header,
|  localhost:8888        localhost:8888             localhost:9004                       inference-api-translator)
+---------------------------+                         +---------------------------+
```

The Go backend is the core — it speaks the real `envoy.service.ext_proc.v3.ExternalProcessor` gRPC protocol, sending `ProcessingRequest` messages (headers + body) and receiving `ProcessingResponse` messages (header mutations + body mutations), exactly like Envoy would. The browser provides a UI for editing inputs and displaying results.

## Prerequisites

- **Go 1.25+**
- **oc** CLI (OpenShift) or **kubectl** — logged into a cluster
- **BBR pod deployed** in the cluster (see [Deploying BBR](#deploying-bbr) below)

## Quick Start

```bash
# 1. Deploy BBR to your cluster (one-time setup)
./deploy/build.sh /path/to/ai-gateway-payload-processing

# 2. Start port-forward (keep this terminal open)
oc port-forward svc/bbr-plugins 9004:9004 -n bbr-plugins

# 3. Run the sandbox (opens browser automatically)
go run .
```

The tool opens at `http://localhost:8888`.

## Usage

### Environment Tab

The Environment tab shows your cluster connection and BBR deployment status.

**Connecting to a cluster:**
1. Enter the API server URL, username, and password
2. Set the BBR namespace (default: `bbr-plugins`)
3. Click **Login to Cluster**

Once connected, you'll see:
- **Architecture diagram** — live status of each component (green = active, red = action needed)
- **Cluster info** — server, user, namespace
- **BBR deployment** — pod name, status, uptime, ports
- **Active plugins** — the plugin chain configured on the BBR pod, in execution order
- **Port-forward instructions** — if not connected, shows the exact command to run with a copy button

**Switching environments:**
Click **Disconnect** to log out, then log in to a different cluster.

### Sandbox Tab

The Sandbox tab is where you test requests and responses against the BBR plugin chain.

**Sending a request:**
1. Select a sample preset (e.g., "Anthropic Translation") or manually edit headers and body
2. Choose **Request** or **Response** phase
3. Click **Send to BBR**

**Results show:**
- **Header Mutations** — headers added (green SET badge) or removed (red DEL badge) by the plugins
- **Body** — the mutated body with changed/added fields highlighted
- **Duration** — round-trip time for the gRPC ext-proc exchange

**Sample presets:**

| Preset | Phase | What it tests |
|---|---|---|
| OpenAI Chat | Request | Basic passthrough (body-field-to-header extracts model name) |
| Anthropic Translation | Request | Full API translation (OpenAI -> Anthropic format) |
| System Messages | Request | System/developer message extraction to Anthropic `system` param |
| Tool Use | Request | Tool definitions in request |
| Anthropic Response | Response | Anthropic -> OpenAI response translation |
| Anthropic Error | Response | Error response format translation |
| OpenAI Response | Response | Passthrough (no mutation expected) |

## CLI Flags

| Flag | Default | Description |
|---|---|---|
| `--port` | `8888` | HTTP server port for the web UI |
| `--namespace` | `bbr-plugins` | Kubernetes namespace where BBR is deployed |
| `--no-browser` | `false` | Don't auto-open the browser on start |

## Deploying BBR

The BBR server runs the plugins from [ai-gateway-payload-processing](https://github.com/opendatahub-io/ai-gateway-payload-processing) in a pod. The `deploy/` directory contains everything needed.

### First-time setup

```bash
# Create the namespace
oc create namespace bbr-plugins

# Apply RBAC (BBR needs cluster-wide ConfigMap read access)
oc apply -f deploy/rbac.yaml

# Build and deploy (pass the path to ai-gateway-payload-processing source)
./deploy/build.sh /path/to/ai-gateway-payload-processing
```

### What gets deployed

- **BuildConfig** — OpenShift binary Docker build (`bbr-plugins`)
- **ImageStream** — `bbr-plugins:latest` in the internal registry
- **Deployment** — single pod running the BBR gRPC server on port 9004
- **Service** — `bbr-plugins` ClusterIP service exposing ports 9004 (gRPC), 9005 (health), 9090 (metrics)
- **RBAC** — ClusterRole/ClusterRoleBinding for ConfigMap access

### Configured plugins

The default deployment runs two plugins (configured via `--plugin` args in `deploy/deployment.yaml`):

1. **body-field-to-header** — extracts `model` from the request body and sets it as `X-Gateway-Model-Name` header
2. **inference-api-translator** — translates between OpenAI and Anthropic API formats

To change the plugin chain, edit the `args` in `deploy/deployment.yaml` and re-apply:
```bash
oc apply -f deploy/deployment.yaml
```

### Rebuilding after code changes

```bash
# Rebuild the BBR image from updated source
oc start-build bbr-plugins --from-dir=/path/to/ai-gateway-payload-processing --follow -n bbr-plugins

# Restart the pod to pick up the new image
oc rollout restart deployment/bbr-plugins -n bbr-plugins
```

## Project Structure

```
bbr-sandbox/
├── main.go                           # Entry point, HTTP server, browser open
├── go.mod
├── Makefile
├── Dockerfile                        # Container image for the sandbox itself
├── deploy/
│   ├── Dockerfile                    # BBR server container image
│   ├── deployment.yaml               # BBR pod + service (bbr-plugins namespace)
│   ├── rbac.yaml                     # ClusterRole for ConfigMap access
│   └── build.sh                      # Build & deploy script
├── internal/
│   ├── extproc/
│   │   └── client.go                 # gRPC ext-proc client (Envoy simulator)
│   ├── cluster/
│   │   └── info.go                   # Cluster info via oc CLI
│   ├── sandbox/
│   │   ├── server.go                 # HTTP server, static file serving
│   │   └── handlers.go               # REST API handlers
│   └── presets/
│       └── presets.go                # Sample request/response templates
└── web/                              # Embedded frontend (dark mode)
    ├── index.html
    ├── css/style.css
    └── js/
        ├── app.js                    # Main controller, tabs, API calls
        ├── editor.js                 # JSON editor with validation
        ├── headers.js                # Key-value header editor
        └── diff.js                   # Color-coded mutation rendering
```

## API Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/health` | gRPC connection status |
| `POST` | `/api/connect` | Connect gRPC client to BBR target |
| `POST` | `/api/send/request` | Send request through BBR ext-proc |
| `POST` | `/api/send/response` | Send response through BBR ext-proc |
| `GET` | `/api/presets` | List sample request/response templates |
| `GET` | `/api/cluster` | Cluster and BBR deployment info (via oc) |
| `POST` | `/api/cluster/login` | Log in to a cluster (oc login) |
| `POST` | `/api/cluster/logout` | Log out of cluster (oc logout) |
