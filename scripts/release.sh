#!/usr/bin/env bash
set -euo pipefail

# Usage: ./scripts/release.sh [major|minor|patch]
BUMP="${1:-patch}"

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Error: working tree has uncommitted changes. Commit or stash them first." >&2
  exit 1
fi

git fetch --tags --quiet

LATEST=$(git tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1)
[[ -z "$LATEST" ]] && LATEST="v0.0.0"

echo "Latest tag: $LATEST"

VERSION="${LATEST#v}"
MAJOR=$(echo "$VERSION" | cut -d. -f1)
MINOR=$(echo "$VERSION" | cut -d. -f2)
PATCH=$(echo "$VERSION" | cut -d. -f3)

case "$BUMP" in
  major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
  minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
  patch) PATCH=$((PATCH + 1)) ;;
  *)
    echo "Error: unknown bump type '$BUMP'. Use major, minor, or patch." >&2
    exit 1
    ;;
esac

NEW_TAG="v${MAJOR}.${MINOR}.${PATCH}"
echo "Creating tag: $NEW_TAG"

git tag "$NEW_TAG"
git push origin "$NEW_TAG"

echo "Released $NEW_TAG"
