#!/usr/bin/env bash
#
# Sync GitHub labels from .github/labels.yml
# Requires: gh (GitHub CLI), yq
#
# Usage:
#   .github/scripts/sync-labels.sh
#   .github/scripts/sync-labels.sh seebom-labs/seebom
#
set -euo pipefail

REPO="${1:-seebom-labs/seebom}"
LABELS_FILE="$(dirname "$0")/../labels.yml"

if ! command -v gh &> /dev/null; then
  echo "❌ GitHub CLI (gh) is required. Install: https://cli.github.com/"
  exit 1
fi

if ! command -v yq &> /dev/null; then
  echo "❌ yq is required. Install: https://github.com/mikefarah/yq"
  exit 1
fi

echo "🏷️  Syncing labels to $REPO from $LABELS_FILE"
echo ""

count=0
total=$(yq '. | length' "$LABELS_FILE")

for i in $(seq 0 $((total - 1))); do
  name=$(yq ".[$i].name" "$LABELS_FILE" -r)
  color=$(yq ".[$i].color" "$LABELS_FILE" -r)
  desc=$(yq ".[$i].description" "$LABELS_FILE" -r)

  if gh label create "$name" --repo "$REPO" --color "$color" --description "$desc" --force 2>/dev/null; then
    echo "  ✅ $name"
  else
    echo "  ⚠️  $name (may already exist)"
  fi
  count=$((count + 1))
done

echo ""
echo "✅ Synced $count labels to $REPO"

