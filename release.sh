#!/bin/bash
export TAG=v3.1.0-rc01
# =============================================================================
# Release script for owlcms-controlpanel
# Uses workflow_dispatch trigger with tag input parameter
# Monitors builds using timestamp-based filtering (owlcms4 approach)
# =============================================================================

set -e

# Check for gh CLI
if ! command -v gh &> /dev/null; then
    echo "Error: gh CLI is not installed. Please install it first."
    exit 1
fi

# Check authentication
if ! gh auth status &> /dev/null; then
    echo "Error: gh CLI is not authenticated. Run 'gh auth login' first."
    exit 1
fi

# Get tag from argument
if [[ -z "$1" ]]; then
    echo "Usage: $0 <tag>"
    echo "Example: $0 1.9.0"
    exit 1
fi
TAG="$1"

# Validate tag format (semver without v prefix)
if [[ ! "$TAG" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-alpha[0-9]*|-beta[0-9]*|-rc[0-9]*)?$ ]]; then
    echo "Error: Tag must be semver format: 1.2.3, 1.2.3-alpha1, 1.2.3-beta1, or 1.2.3-rc1"
    exit 1
fi

# Get current branch
BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [[ "$BRANCH" == "HEAD" ]]; then
    echo "Error: Detached HEAD state. Checkout a branch before releasing."
    exit 1
fi

REPO="owlcms/owlcms-controlpanel"
WORKFLOW_FILE="release.yaml"

echo "==== Release $TAG for $REPO ===="
echo "Branch: $BRANCH"

# Get the most recent run ID before triggering (for comparison)
PREV_RUN_ID=$(gh run list --repo "$REPO" --workflow "$WORKFLOW_FILE" --limit 1 --json databaseId --jq '.[0].databaseId // 0')
START_ISO=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
echo "Previous run ID: $PREV_RUN_ID, Start time: $START_ISO"

# Trigger the workflow
echo "Triggering workflow dispatch with tag=$TAG on branch $BRANCH..."
gh workflow run "$WORKFLOW_FILE" --repo "$REPO" --ref "$BRANCH" --field tag="$TAG"

# Wait for the new run to appear
echo "Waiting for workflow run to start..."
sleep 10

# Find the new run (started after our timestamp, or with ID > previous)
for i in {1..30}; do
    RUN_ID=$(gh run list --repo "$REPO" --workflow "$WORKFLOW_FILE" --event workflow_dispatch --limit 5 --json databaseId,createdAt --jq "[.[] | select(.createdAt >= \"$START_ISO\" or .databaseId > $PREV_RUN_ID)] | .[0].databaseId // 0")
    
    if [[ "$RUN_ID" != "0" && -n "$RUN_ID" ]]; then
        echo "Found new workflow run: $RUN_ID"
        break
    fi
    echo "Waiting for run to appear (attempt $i/30)..."
    sleep 5
done

if [[ "$RUN_ID" == "0" || -z "$RUN_ID" ]]; then
    echo "Error: Could not find new workflow run after 150 seconds"
    echo "Check https://github.com/$REPO/actions"
    exit 1
fi

# Monitor the workflow
echo ""
echo "==== Monitoring workflow run $RUN_ID ===="
echo "View at: https://github.com/$REPO/actions/runs/$RUN_ID"
echo ""

gh run watch "$RUN_ID" --repo "$REPO" --exit-status

echo ""
echo "==== Release $TAG completed successfully! ===="
echo "View release at: https://github.com/$REPO/releases/tag/$TAG"
