#!/bin/bash
# Run external model E2E tests locally against a RHOAI cluster.
#
# Prerequisites:
#   - oc logged into the cluster
#   - MaaS deployed with ExternalModel reconciler
#   - BBR deployed with all plugins
#   - External simulator accessible
#
# Usage:
#   ./run-external-model-tests.sh [gateway-host] [simulator-endpoint]
#
# Examples:
#   ./run-external-model-tests.sh maas.apps.ocp.4fnz2.sandbox2228.opentlc.com
#   ./run-external-model-tests.sh maas.apps.cluster.example.com 10.0.0.50

set -euo pipefail

GATEWAY_HOST="${1:-${GATEWAY_HOST:-}}"
SIMULATOR_ENDPOINT="${2:-${E2E_SIMULATOR_ENDPOINT:-3.150.113.9}}"

if [[ -z "$GATEWAY_HOST" ]]; then
    echo "Usage: $0 <gateway-host> [simulator-endpoint]"
    echo "  Or set GATEWAY_HOST env var"
    exit 1
fi

# Verify oc login
if ! oc whoami &>/dev/null; then
    echo "ERROR: Not logged into OpenShift. Run 'oc login' first."
    exit 1
fi

export GATEWAY_HOST
export E2E_SIMULATOR_ENDPOINT="$SIMULATOR_ENDPOINT"
export E2E_SKIP_TLS_VERIFY="true"
export TOKEN=$(oc whoami -t)

echo "=== External Model E2E Tests ==="
echo "  Gateway:   $GATEWAY_HOST"
echo "  Simulator: $E2E_SIMULATOR_ENDPOINT"
echo "  Token:     ${TOKEN:0:20}..."
echo ""

# Find the MaaS repo
MAAS_DIR="${MAAS_REPO:-$(dirname "$0")/../../models-as-a-service}"
if [[ ! -d "$MAAS_DIR/test/e2e" ]]; then
    echo "ERROR: MaaS repo not found at $MAAS_DIR"
    echo "  Set MAAS_REPO env var or clone models-as-a-service next to tools/"
    exit 1
fi

cd "$MAAS_DIR/test/e2e"

# Setup venv
if [[ ! -d .venv ]]; then
    echo "Creating Python venv..."
    python3 -m venv .venv
fi
source .venv/bin/activate
pip install -q -r requirements.txt

# Run tests
echo ""
echo "Running external model tests..."
pytest tests/test_external_models.py -v \
    --html=reports/external-models.html \
    --junitxml=reports/external-models-junit.xml \
    "$@"
