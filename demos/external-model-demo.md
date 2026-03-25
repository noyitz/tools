# ExternalModel Demo — Add a New Model and Run Inference

This demo shows how to add a new external model to MaaS, watch the controller
auto-create Istio routing resources, configure auth, and run inference through
the BBR (Body-Based Router) pipeline.

**Prerequisites**:
- Logged into the OCP cluster (`oc login`)
- MaaS deployed with the ExternalModel reconciler (PR #582)
- BBR pod running with `body-field-to-header`, `provider-resolver`, and
  `api-translation` plugins

## Environment

```bash
# Set to your MaaS gateway URL
HOST="https://maas.apps.<your-cluster-domain>"
```

---

## Step 1 — Show Current State

```bash
oc get maasmodelref -n llm
```

## Step 2 — Create a Secret for the New Model

```bash
oc create secret generic anthropic-api-key \
  -n llm \
  --from-literal=api-key=demo-key

oc label secret anthropic-api-key -n llm \
  inference.networking.k8s.io/bbr-managed=true
```

## Step 3 — Create the MaaSModelRef

```bash
cat > /tmp/claude-sonnet.yaml <<'EOF'
apiVersion: maas.opendatahub.io/v1alpha1
kind: MaaSModelRef
metadata:
  name: claude-sonnet
  namespace: llm
  annotations:
    maas.opendatahub.io/endpoint: "<simulator-ip>"
    maas.opendatahub.io/provider: "anthropic"
spec:
  credentialRef:
    name: anthropic-api-key
    namespace: llm
  modelRef:
    name: claude-sonnet
    kind: ExternalModel
    provider: anthropic
    endpoint: <simulator-ip>
EOF

oc apply -f /tmp/claude-sonnet.yaml
```

## Step 4 — Watch Istio Resources Auto-Create (~5 seconds)

The ExternalModel reconciler (PR #582) creates these in `openshift-ingress`:

```bash
echo "=== MaaSModelRef ==="
oc get maasmodelref claude-sonnet -n llm

echo "=== HTTPRoutes ==="
oc get httproute -n openshift-ingress | grep anthropic

echo "=== Services ==="
oc get svc -n openshift-ingress | grep anthropic

echo "=== ServiceEntries ==="
oc get serviceentry -n openshift-ingress | grep anthropic

echo "=== DestinationRules ==="
oc get destinationrule -n openshift-ingress | grep anthropic
```

The model should show `PHASE: Ready`.

## Step 5 — Create the Backend Service

The `maas-model-*` HTTPRoute (Step 6) needs a local service in the `llm`
namespace that routes to the external provider.

```bash
oc apply -f - <<'EOF'
apiVersion: v1
kind: Service
metadata:
  name: anthropic-simulator
  namespace: llm
spec:
  type: ExternalName
  externalName: ai-simulator.external
  ports:
  - port: 8000
    protocol: TCP
EOF
```

> **Note for PR #582**: The reconciler should create this service in the model
> namespace, not just in the gateway namespace.

## Step 6 — Create the Model HTTPRoute

The MaaS auth/subscription controllers need a `maas-model-<name>` HTTPRoute in
the model namespace. BBR sets the `X-Gateway-Model-Name` header from the
request body and calls ClearRouteCache, which makes Envoy re-evaluate and
match this header-based route.

```bash
oc apply -f - <<'EOF'
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: maas-model-claude-sonnet
  namespace: llm
spec:
  parentRefs:
  - name: maas-default-gateway
    namespace: openshift-ingress
  rules:
  - matches:
    - headers:
      - name: X-Gateway-Model-Name
        type: Exact
        value: claude-sonnet
    backendRefs:
    - name: anthropic-simulator
      port: 8000
EOF
```

No path matching or URLRewrite needed — BBR handles path rewriting via the
api-translation plugin.

> **Note for PR #582**: The reconciler should also create this `maas-model-*`
> HTTPRoute in the model namespace. Without it, the MaaS auth policy and
> subscription controllers fail to resolve the route for the model.

## Step 7 — Add to Auth Policy and Subscription

The model needs to be added to a MaaSAuthPolicy and MaaSSubscription before
inference is allowed.

```bash
oc patch maasauthpolicy external-models-access \
  -n models-as-a-service --type=merge -p '{
  "spec": {
    "modelRefs": [
      {"name": "gpt-4o", "namespace": "llm"},
      {"name": "claude-sonnet", "namespace": "llm"}
    ]
  }
}'

oc patch maassubscription external-models-subscription \
  -n models-as-a-service --type=merge -p '{
  "spec": {
    "modelRefs": [
      {"name": "gpt-4o", "namespace": "llm",
       "tokenRateLimits": [{"limit": 10000, "window": "1m"}]},
      {"name": "claude-sonnet", "namespace": "llm",
       "tokenRateLimits": [{"limit": 10000, "window": "1m"}]}
    ]
  }
}'
```

Wait ~5 seconds for the auth policy to propagate:

```bash
oc get maasauthpolicy external-models-access \
  -n models-as-a-service -o jsonpath='{.status.phase}'
# Should show: Active
```

## Step 8 — Get an API Key

```bash
TOKEN=$(oc whoami -t)
API_KEY=$(curl -sSk -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"demo-key","expiresIn":"2h"}' \
  "$HOST/maas-api/v1/api-keys" | jq -r '.key')
echo "API Key: $API_KEY"
```

## Step 9 — Run Inference

Send a standard OpenAI Chat Completions request. BBR handles the full flow:
1. `body-field-to-header` extracts `model` from body → sets
   `X-Gateway-Model-Name: claude-sonnet` header
2. ClearRouteCache → Envoy re-matches to `maas-model-claude-sonnet` HTTPRoute
3. `provider-resolver` resolves provider from MaaSModelRef → writes to
   CycleState
4. `api-translation` translates OpenAI format → Anthropic format
5. Request reaches the backend

```bash
curl -sk -X POST "$HOST/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "claude-sonnet",
    "messages": [
      {"role": "user", "content": "Say hello in one word"}
    ]
  }'
```

Expected: A 200 response with a chat completion.

**Supported providers** for `spec.modelRef.provider`:
- `openai`
- `anthropic`
- `azure-openai`
- `vertex`

## Step 10 — Clean Up

```bash
# Remove the model (finalizer auto-deletes Istio resources in openshift-ingress)
oc delete maasmodelref claude-sonnet -n llm

# Remove manual resources
oc delete httproute maas-model-claude-sonnet -n llm
oc delete svc anthropic-simulator -n llm
oc delete secret anthropic-api-key -n llm

# Remove from auth policy and subscription (patch back to just gpt-4o)
oc patch maasauthpolicy external-models-access \
  -n models-as-a-service --type=merge -p '{
  "spec": {
    "modelRefs": [
      {"name": "gpt-4o", "namespace": "llm"}
    ]
  }
}'

oc patch maassubscription external-models-subscription \
  -n models-as-a-service --type=merge -p '{
  "spec": {
    "modelRefs": [
      {"name": "gpt-4o", "namespace": "llm",
       "tokenRateLimits": [{"limit": 10000, "window": "1m"}]}
    ]
  }
}'
```

---

## How It Works

```
User Request (OpenAI format)
    │
    ▼
Gateway (Envoy)
    │
    ▼
BBR ext_proc
    ├── body-field-to-header: extracts "model" → X-Gateway-Model-Name header
    ├── ClearRouteCache → Envoy re-matches to maas-model-* HTTPRoute
    ├── provider-resolver: MaaSModelRef → provider/creds in CycleState
    └── api-translation: OpenAI → provider-native format (e.g. Anthropic)
    │
    ▼
maas-model-* HTTPRoute (header match: X-Gateway-Model-Name)
    │
    ▼
Backend Service → External Provider (via Istio egress)
    │
    ▼
Response flows back through BBR (api-translation: provider → OpenAI format)
    │
    ▼
User Response (OpenAI format)
```

## Notes

- The ExternalModel reconciler (PR #582) creates Istio egress resources
  (Service, ServiceEntry, DestinationRule, HTTPRoute) in the **gateway
  namespace** (`openshift-ingress`).
- The `maas-model-*` HTTPRoute in the model namespace uses **header-based
  matching only** (`X-Gateway-Model-Name`). No path matching or URLRewrite —
  BBR handles path rewriting via the api-translation plugin.
- Auth requires a MaaSAuthPolicy + MaaSSubscription covering the model, plus
  a valid API key.
