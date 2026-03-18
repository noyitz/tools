#!/bin/bash
set -euo pipefail

NAMESPACE="bbr-plugins"
BUILD_NAME="bbr-plugins"
SOURCE_DIR="${1:-/Users/nitzikow/code/ai-gateway-payload-processing}"

echo "=== BBR Plugins Build & Deploy ==="
echo "Source: $SOURCE_DIR"
echo "Namespace: $NAMESPACE"
echo ""

# Create BuildConfig + ImageStream if they don't exist
if ! oc get buildconfig "$BUILD_NAME" -n "$NAMESPACE" &>/dev/null; then
  echo "Creating BuildConfig and ImageStream..."
  oc new-build --binary --name="$BUILD_NAME" \
    --strategy=docker \
    --dockerfile="$(cat "$(dirname "$0")/Dockerfile")" \
    -n "$NAMESPACE"
else
  echo "BuildConfig '$BUILD_NAME' already exists."
fi

# Build and push image
echo ""
echo "Starting build from $SOURCE_DIR..."
oc start-build "$BUILD_NAME" --from-dir="$SOURCE_DIR" --follow -n "$NAMESPACE"

# Apply deployment
echo ""
echo "Applying deployment..."
oc apply -f "$(dirname "$0")/deployment.yaml"

# Wait for rollout
echo ""
echo "Waiting for rollout..."
oc rollout status deployment/"$BUILD_NAME" -n "$NAMESPACE" --timeout=120s

echo ""
echo "=== Done ==="
echo "BBR plugins pod is running. Connect with:"
echo "  oc port-forward svc/$BUILD_NAME 9004:9004 -n $NAMESPACE"
