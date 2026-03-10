#!/usr/bin/env bash
set -euo pipefail

# Sync GitHub labels from .github/labels.yml
# Requires: gh (GitHub CLI), yq (YAML parser)
#
# Usage: .github/scripts/sync-labels.sh
#        make sync-labels

LABELS_FILE=".github/labels.yml"

if ! command -v gh &>/dev/null; then
  echo "❌ gh (GitHub CLI) is required. Install: https://cli.github.com"
  exit 1
fi

if ! command -v yq &>/dev/null; then
  echo "❌ yq is required. Install: brew install yq / go install github.com/mikefarah/yq/v4@latest"
  exit 1
fi

REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null)
if [[ -z "$REPO" ]]; then
  echo "❌ Could not detect repo. Run from inside a git repo with gh auth."
  exit 1
fi

echo "🏷️  Syncing labels to $REPO from $LABELS_FILE"

COUNT=$(yq 'length' "$LABELS_FILE")
for i in $(seq 0 $((COUNT - 1))); do
  NAME=$(yq ".[$i].name" "$LABELS_FILE")
  COLOR=$(yq ".[$i].color" "$LABELS_FILE")
  DESC=$(yq ".[$i].description" "$LABELS_FILE")

  if gh label list --repo "$REPO" --search "$NAME" --json name -q '.[].name' 2>/dev/null | grep -qx "$NAME"; then
    gh label edit "$NAME" --repo "$REPO" --color "$COLOR" --description "$DESC" 2>/dev/null
    echo "  ✏️  Updated: $NAME"
  else
    gh label create "$NAME" --repo "$REPO" --color "$COLOR" --description "$DESC" 2>/dev/null
    echo "  ✅ Created: $NAME"
  fi
done

echo "🏷️  Done. $COUNT labels synced."

