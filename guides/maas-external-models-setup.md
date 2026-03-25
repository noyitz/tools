# MaaS External Models — Setup & Testing Guide

This guide covers what needs to be done **after MaaS is deployed** to enable
external model support with BBR (Body-Based Router) payload processing plugins.

## Prerequisites

- MaaS deployed on RHOAI/OCP — see the [MaaS deployment guide](https://github.com/opendatahub-io/models-as-a-service/blob/main/docs/README.md) and [`deploy.sh`](https://github.com/opendatahub-io/models-as-a-service/blob/main/scripts/deploy.sh)
- Kuadrant + Authorino + Limitador running
- At least one internal model (LLMInferenceService) deployed
- An external AI simulator or real provider endpoint (see [llm-katan](https://github.com/yossiovadia/llm-katan) by @yossiovadia — a drop-in test server that supports OpenAI, Anthropic, Bedrock, and Vertex APIs with real inference, no API keys or GPU needed)

## Overview

```
┌────────────────────────────────────────────────────────────────┐
│                     What the reconciler creates                │
│                     (auto, in openshift-ingress)               │
│  ┌──────────────┐ ┌──────────────┐ ┌───────────────────────┐  │
│  │ ExternalName  │ │ ServiceEntry │ │ DestinationRule (TLS) │  │
│  │ Service       │ │              │ │                       │  │
│  └──────────────┘ └──────────────┘ └───────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ HTTPRoute (path: /external/<provider>/)                  │  │
│  └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│                What you create manually                        │
│                (until reconciler PR #582 is updated)           │
│  In model namespace (e.g., llm):                               │
│  ┌──────────────┐ ┌──────────────────────────────────────────┐ │
│  │ ExternalName  │ │ maas-model-* HTTPRoute                  │ │
│  │ Service       │ │  - path match: /<model> (for auth)      │ │
│  │ (simulator)   │ │  - header match: X-Gateway-Model-Name   │ │
│  └──────────────┘ │    (for BBR routing)                     │ │
│                    └──────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────┘
```

---

## Step 1 — Build and Deploy the Custom MaaS Controller

The custom MaaS controller includes PR #582 (ExternalModel reconciler) and
PR #571 (ExternalModel CRD fields).

```bash
# From the models-as-a-service repo, build the custom controller image
cd models-as-a-service

# Start a binary build (requires an existing BuildConfig)
oc start-build maas-controller-custom --from-dir=maas-controller/ \
  -n redhat-ods-applications --follow

# Patch the deployment to use the custom image
oc set image deployment/maas-controller \
  manager=image-registry.openshift-image-registry.svc:5000/redhat-ods-applications/maas-controller-custom:latest \
  -n redhat-ods-applications

# Restart to pick up the new image
oc rollout restart deployment/maas-controller -n redhat-ods-applications
oc rollout status deployment/maas-controller -n redhat-ods-applications --timeout=120s

# Apply the updated RBAC (needed for Istio resources)
oc apply -f deployment/base/maas-controller/rbac/clusterrole.yaml
```

## Step 2 — Build and Deploy the BBR Plugins

The BBR pod runs the payload processing plugins: `body-field-to-header`,
`provider-resolver`, `api-translation`, and `apikey-injection`.

```bash
# From the ai-gateway-payload-processing repo
cd ai-gateway-payload-processing

# Build and push (requires an existing BuildConfig)
oc start-build bbr-plugins --from-dir=. \
  -n redhat-ods-applications --follow

# Restart to pick up the new image
oc rollout restart deployment/bbr-plugins -n redhat-ods-applications
oc rollout status deployment/bbr-plugins -n redhat-ods-applications --timeout=60s
```

**BBR deployment args** (in the Deployment spec):

```yaml
args:
  - --plugin=body-field-to-header:model-header:{"field_name":"model","header_name":"X-Gateway-Model-Name"}
  - --plugin=model-provider-resolver:default
  - --plugin=api-translation:default
  - --plugin=apikey-injection:default
```

## Step 3 — Create the BBR EnvoyFilter

The EnvoyFilter wires BBR as an ext_proc filter in the Envoy proxy.

```bash
oc apply -f - <<'EOF'
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: bbr-ext-proc
  namespace: openshift-ingress
spec:
  configPatches:
  - applyTo: HTTP_FILTER
    match:
      listener:
        filterChain:
          filter:
            name: envoy.filters.network.http_connection_manager
            subFilter:
              name: envoy.filters.http.router
    patch:
      operation: INSERT_BEFORE
      value:
        name: envoy.filters.http.ext_proc
        typed_config:
          '@type': type.googleapis.com/envoy.extensions.filters.http.ext_proc.v3.ExternalProcessor
          failure_mode_allow: true
          grpc_service:
            envoy_grpc:
              cluster_name: outbound|9004||bbr-plugins.redhat-ods-applications.svc.cluster.local
            timeout: 10s
          processing_mode:
            request_header_mode: SEND
            response_header_mode: SEND
            request_body_mode: BUFFERED
            response_body_mode: BUFFERED
EOF
```

**Also required** — NetworkPolicy and DestinationRule for BBR connectivity:

```bash
# NetworkPolicy to allow gRPC on port 9004
oc apply -f - <<'EOF'
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: bbr-plugins-allow-grpc
  namespace: redhat-ods-applications
spec:
  podSelector:
    matchLabels:
      app: bbr-plugins
  ingress:
  - ports:
    - port: 9004
      protocol: TCP
    - port: 9005
      protocol: TCP
    - port: 9090
      protocol: TCP
EOF

# DestinationRule for HTTP/2 (required for gRPC)
oc apply -f - <<'EOF'
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: bbr-plugins-h2c
  namespace: redhat-ods-applications
spec:
  host: bbr-plugins.redhat-ods-applications.svc.cluster.local
  trafficPolicy:
    connectionPool:
      http:
        h2UpgradePolicy: UPGRADE
EOF
```

## Step 4 — Add External Models

For each external model, create: Secret → MaaSModelRef → simulator Service →
maas-model HTTPRoute → add to auth policy + subscription.

### 4a. Create the Secret

```bash
oc create secret generic <provider>-api-key \
  -n llm --from-literal=api-key=<key-value>

oc label secret <provider>-api-key -n llm \
  inference.networking.k8s.io/bbr-managed=true
```

### 4b. Create the MaaSModelRef

```bash
oc apply -f - <<'EOF'
apiVersion: maas.opendatahub.io/v1alpha1
kind: MaaSModelRef
metadata:
  name: <model-name>
  namespace: llm
  annotations:
    maas.opendatahub.io/endpoint: "<provider-ip-or-fqdn>"
    maas.opendatahub.io/provider: "<provider>"
spec:
  credentialRef:
    name: <provider>-api-key
    namespace: llm
  modelRef:
    name: <model-name>
    kind: ExternalModel
    provider: <provider>
    endpoint: <provider-ip-or-fqdn>
EOF
```

The reconciler auto-creates Service, ServiceEntry, DestinationRule, and
HTTPRoute in `openshift-ingress`. Verify with:

```bash
oc get maasmodelref <model-name> -n llm
# Should show PHASE: Ready
```

### 4c. Create Simulator Service (manual — until reconciler is updated)

The `maas-model-*` HTTPRoute needs a backend service in the model namespace:

```bash
oc apply -f - <<'EOF'
apiVersion: v1
kind: Service
metadata:
  name: <provider>-simulator
  namespace: llm
spec:
  type: ExternalName
  externalName: 3.150.113.9
  ports:
  - port: 8000
    protocol: TCP
EOF
```

### 4d. Create maas-model HTTPRoute (manual — until reconciler is updated)

This HTTPRoute needs **both** match rules:
- **Path-based**: So the Kuadrant Wasm plugin can enforce auth + rate limiting
  (the Wasm plugin runs before BBR in the Envoy filter chain)
- **Header-based**: For BBR's ClearRouteCache routing after setting
  `X-Gateway-Model-Name`

```bash
oc apply -f - <<'EOF'
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: maas-model-<model-name>
  namespace: llm
spec:
  parentRefs:
  - name: maas-default-gateway
    namespace: openshift-ingress
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /<model-name>
    backendRefs:
    - name: <provider>-simulator
      port: 8000
    filters:
    - type: URLRewrite
      urlRewrite:
        path:
          replacePrefixMatch: /
          type: ReplacePrefixMatch
  - matches:
    - headers:
      - name: X-Gateway-Model-Name
        type: Exact
        value: <model-name>
    backendRefs:
    - name: <provider>-simulator
      port: 8000
EOF
```

### 4e. Add to Auth Policy and Subscription

```bash
# Add to MaaSAuthPolicy (include ALL existing models)
oc patch maasauthpolicy external-models-access \
  -n models-as-a-service --type=merge -p '{
  "spec": {"modelRefs": [
    {"name": "gpt-4o", "namespace": "llm"},
    {"name": "claude-sonnet", "namespace": "llm"},
    {"name": "<new-model>", "namespace": "llm"}
  ]}
}'

# Add to MaaSSubscription (include ALL existing models)
oc patch maassubscription external-models-subscription \
  -n models-as-a-service --type=merge -p '{
  "spec": {"modelRefs": [
    {"name": "gpt-4o", "namespace": "llm",
     "tokenRateLimits": [{"limit": 10000, "window": "1m"}]},
    {"name": "claude-sonnet", "namespace": "llm",
     "tokenRateLimits": [{"limit": 10000, "window": "1m"}]},
    {"name": "<new-model>", "namespace": "llm",
     "tokenRateLimits": [{"limit": 10000, "window": "1m"}]}
  ]}
}'
```

Wait ~10 seconds, then verify:

```bash
oc get maasauthpolicy external-models-access \
  -n models-as-a-service -o jsonpath='{.status.phase}'
# Should show: Active
```

## Step 5 — Complete Example: All 4 Providers

```bash
# --- Secrets ---
oc create secret generic openai-api-key -n llm --from-literal=api-key=sk-openai-demo-1234
oc label secret openai-api-key -n llm inference.networking.k8s.io/bbr-managed=true

oc create secret generic anthropic-api-key -n llm --from-literal=api-key=sk-ant-demo-5678
oc label secret anthropic-api-key -n llm inference.networking.k8s.io/bbr-managed=true

oc create secret generic azure-openai-api-key -n llm --from-literal=api-key=az-openai-demo-9012
oc label secret azure-openai-api-key -n llm inference.networking.k8s.io/bbr-managed=true

oc create secret generic vertex-api-key -n llm --from-literal=api-key=vtx-demo-3456
oc label secret vertex-api-key -n llm inference.networking.k8s.io/bbr-managed=true

# --- MaaSModelRefs ---
cat > /tmp/external-models.yaml <<'EOF'
apiVersion: maas.opendatahub.io/v1alpha1
kind: MaaSModelRef
metadata:
  name: gpt-4o
  namespace: llm
  annotations:
    maas.opendatahub.io/endpoint: "3.150.113.9"
    maas.opendatahub.io/provider: "openai"
spec:
  credentialRef: {name: openai-api-key, namespace: llm}
  modelRef: {name: gpt-4o, kind: ExternalModel, provider: openai, endpoint: 3.150.113.9}
---
apiVersion: maas.opendatahub.io/v1alpha1
kind: MaaSModelRef
metadata:
  name: claude-sonnet
  namespace: llm
  annotations:
    maas.opendatahub.io/endpoint: "3.150.113.9"
    maas.opendatahub.io/provider: "anthropic"
spec:
  credentialRef: {name: anthropic-api-key, namespace: llm}
  modelRef: {name: claude-sonnet, kind: ExternalModel, provider: anthropic, endpoint: 3.150.113.9}
---
apiVersion: maas.opendatahub.io/v1alpha1
kind: MaaSModelRef
metadata:
  name: gpt-4o-azure
  namespace: llm
  annotations:
    maas.opendatahub.io/endpoint: "3.150.113.9"
    maas.opendatahub.io/provider: "azure-openai"
spec:
  credentialRef: {name: azure-openai-api-key, namespace: llm}
  modelRef: {name: gpt-4o-azure, kind: ExternalModel, provider: azure-openai, endpoint: 3.150.113.9}
---
apiVersion: maas.opendatahub.io/v1alpha1
kind: MaaSModelRef
metadata:
  name: gemini-flash
  namespace: llm
  annotations:
    maas.opendatahub.io/endpoint: "3.150.113.9"
    maas.opendatahub.io/provider: "vertex"
spec:
  credentialRef: {name: vertex-api-key, namespace: llm}
  modelRef: {name: gemini-flash, kind: ExternalModel, provider: vertex, endpoint: 3.150.113.9}
EOF

oc apply -f /tmp/external-models.yaml

# Wait for reconciler
sleep 5
oc get maasmodelref -n llm

# --- Simulator Services (manual) ---
for name in openai anthropic azure-openai vertex; do
cat <<EOF | oc apply -f -
apiVersion: v1
kind: Service
metadata:
  name: ${name}-simulator
  namespace: llm
spec:
  type: ExternalName
  externalName: 3.150.113.9
  ports:
  - port: 8000
    protocol: TCP
EOF
done

# --- maas-model-* HTTPRoutes (manual) ---
for pair in "gpt-4o:openai" "claude-sonnet:anthropic" "gpt-4o-azure:azure-openai" "gemini-flash:vertex"; do
  model=$(echo $pair | cut -d: -f1)
  provider=$(echo $pair | cut -d: -f2)
cat <<EOF | oc apply -f -
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: maas-model-${model}
  namespace: llm
spec:
  parentRefs:
  - name: maas-default-gateway
    namespace: openshift-ingress
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /${model}
    backendRefs:
    - name: ${provider}-simulator
      port: 8000
    filters:
    - type: URLRewrite
      urlRewrite:
        path:
          replacePrefixMatch: /
          type: ReplacePrefixMatch
  - matches:
    - headers:
      - name: X-Gateway-Model-Name
        type: Exact
        value: ${model}
    backendRefs:
    - name: ${provider}-simulator
      port: 8000
EOF
done

# --- Auth Policy + Subscription ---
oc apply -f - <<'EOF'
apiVersion: maas.opendatahub.io/v1alpha1
kind: MaaSAuthPolicy
metadata:
  name: external-models-access
  namespace: models-as-a-service
spec:
  modelRefs:
  - {name: gpt-4o, namespace: llm}
  - {name: claude-sonnet, namespace: llm}
  - {name: gpt-4o-azure, namespace: llm}
  - {name: gemini-flash, namespace: llm}
  subjects:
    groups:
    - name: system:authenticated
---
apiVersion: maas.opendatahub.io/v1alpha1
kind: MaaSSubscription
metadata:
  name: external-models-subscription
  namespace: models-as-a-service
spec:
  owner:
    groups:
    - name: system:authenticated
  modelRefs:
  - {name: gpt-4o, namespace: llm, tokenRateLimits: [{limit: 10000, window: 1m}]}
  - {name: claude-sonnet, namespace: llm, tokenRateLimits: [{limit: 10000, window: 1m}]}
  - {name: gpt-4o-azure, namespace: llm, tokenRateLimits: [{limit: 10000, window: 1m}]}
  - {name: gemini-flash, namespace: llm, tokenRateLimits: [{limit: 10000, window: 1m}]}
EOF
```

## Step 6 — Test

```bash
HOST="https://<your-gateway-host>"

# Get API key
TOKEN=$(oc whoami -t)
API_KEY=$(curl -sSk -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-key","expiresIn":"2h"}' \
  "$HOST/maas-api/v1/api-keys" | jq -r '.key')
echo "API Key: $API_KEY"

# Test all models
echo "=== gpt-4o (OpenAI) ==="
curl -sk "$HOST/gpt-4o/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}' | jq .

echo "=== claude-sonnet (Anthropic) ==="
curl -sk "$HOST/claude-sonnet/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"model":"claude-sonnet","messages":[{"role":"user","content":"hello"}]}' | jq .

echo "=== gpt-4o-azure (Azure OpenAI) ==="
curl -sk "$HOST/gpt-4o-azure/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"model":"gpt-4o-azure","messages":[{"role":"user","content":"hello"}]}' | jq .

echo "=== gemini-flash (Vertex AI) ==="
curl -sk "$HOST/gemini-flash/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"model":"gemini-flash","messages":[{"role":"user","content":"hello"}]}' | jq .

echo "=== facebook-opt-125m-simulated (Internal KServe) ==="
curl -sk "$HOST/llm/facebook-opt-125m-simulated/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"model":"facebook-opt-125m-simulated","messages":[{"role":"user","content":"hello"}]}' | jq .

# Test auth enforcement
echo "=== Invalid API key (should be 403) ==="
curl -sk -w "\nHTTP_CODE:%{http_code}\n" "$HOST/claude-sonnet/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer INVALID-KEY" \
  -d '{"model":"claude-sonnet","messages":[{"role":"user","content":"hello"}]}'
```

## Supported Providers

| Provider | `spec.modelRef.provider` | API Translation |
|----------|--------------------------|-----------------|
| OpenAI | `openai` | Passthrough (native format) |
| Anthropic | `anthropic` | OpenAI → Anthropic Messages API |
| Azure OpenAI | `azure-openai` | OpenAI → Azure deployment format |
| Vertex AI | `vertex` | OpenAI → Gemini format |

## What the Reconciler Creates (automatic)

When you create a MaaSModelRef with `kind: ExternalModel`, the reconciler
(PR #582) automatically creates in `openshift-ingress`:

| Resource | Name Pattern | Purpose |
|----------|-------------|---------|
| Service (ExternalName) | `<provider>-<ip>-external` | DNS bridge to external endpoint |
| ServiceEntry | `<provider>-<ip>-serviceentry` | Registers host in Istio mesh |
| DestinationRule | `<provider>-<ip>-destinationrule` | TLS origination |
| HTTPRoute | `<provider>-<ip>-httproute` | Routes `/external/<provider>/` path |

Cleanup is automatic via finalizer when the MaaSModelRef is deleted.

## What You Create Manually (until reconciler is updated)

These resources are needed in the **model namespace** (`llm`) but the
reconciler doesn't create them yet (see PR #582 comments):

| Resource | Name Pattern | Purpose |
|----------|-------------|---------|
| Service (ExternalName) | `<provider>-simulator` | Backend ref for the maas-model HTTPRoute |
| HTTPRoute | `maas-model-<model-name>` | Path match (auth) + header match (BBR routing) |

Without the `maas-model-*` HTTPRoute:
- MaaSAuthPolicy fails to resolve the route → no auth enforcement
- MaaSSubscription fails to create TRLP → no rate limiting
- Inference via `/<model-name>/v1/chat/completions` returns 404

## Architecture Notes

### BBR Plugin Chain

```
Request body → body-field-to-header (extract model → X-Gateway-Model-Name)
             → provider-resolver (MaaSModelRef → provider + creds in CycleState)
             → api-translation (OpenAI → provider-native format)
             → apikey-injection (inject provider API key from Secret)
```

### Auth Flow

The Kuadrant Wasm plugin runs **before** BBR in the Envoy filter chain. Auth
is enforced via **path-based predicates** on the `maas-model-*` HTTPRoute:

```
Request: POST /<model>/v1/chat/completions
  → Wasm plugin: predicate matches on path → calls Authorino
  → Authorino: validates API key + subscription for this model
  → Limitador: checks token rate limits
  → BBR: parses body, translates, injects API key
  → Router: forwards to backend
```

This is why the `maas-model-*` HTTPRoute needs a **path-based match rule** —
without it, the Wasm plugin can't match the request and auth is bypassed.

### Known Issues

1. **Auth bypass without model in URL**: Sending `POST /v1/chat/completions`
   (no model in path) bypasses auth because the Wasm plugin runs before BBR
   sets the `X-Gateway-Model-Name` header. Always include the model in the
   URL path until this is resolved.

2. **Reconciler gaps (PR #582)**: The reconciler needs to also create the
   `maas-model-*` HTTPRoute and backend service in the model namespace.
   See inline comments on the PR.

3. **Internal model name matching**: The KServe `--served-model-name` must
   match the MaaSModelRef name. If vLLM registers as `facebook/opt-125m` but
   the MaaSModelRef is `facebook-opt-125m-simulated`, the model server returns
   404. Add `--served-model-name <maas-name>` to the vLLM args.
