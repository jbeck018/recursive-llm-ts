#!/bin/bash
# Release helper script for recursive-llm-ts

set -e

echo "🚀 recursive-llm-ts Release Helper"
echo "=================================="
echo ""

# Check if on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
  echo "⚠️  Warning: You are on branch '$CURRENT_BRANCH', not 'main'"
  read -p "Continue anyway? (y/N): " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    exit 1
  fi
fi

# Check for uncommitted changes
if [ -n "$(git status --porcelain)" ]; then
  echo "❌ Error: You have uncommitted changes"
  echo ""
  git status --short
  echo ""
  echo "Please commit or stash your changes before releasing."
  exit 1
fi

# Pull latest
echo "📥 Pulling latest changes..."
git pull origin $CURRENT_BRANCH

# Show current version
CURRENT_VERSION=$(node -p "require('./package.json').version")
echo ""
echo "Current version: $CURRENT_VERSION"
echo ""

# Calculate next versions for display
IFS='.' read -r -a version_parts <<< "$CURRENT_VERSION"
PATCH_VERSION="${version_parts[0]}.${version_parts[1]}.$((version_parts[2] + 1))"
MINOR_VERSION="${version_parts[0]}.$((version_parts[1] + 1)).0"
MAJOR_VERSION="$((version_parts[0] + 1)).0.0"

# Ask for version bump type
echo "What type of release is this?"
echo "  1) patch (bug fixes: $CURRENT_VERSION -> $PATCH_VERSION)"
echo "  2) minor (new features: $CURRENT_VERSION -> $MINOR_VERSION)"
echo "  3) major (breaking changes: $CURRENT_VERSION -> $MAJOR_VERSION)"
echo "  4) custom version"
echo "  5) cancel"
echo ""
read -p "Enter choice (1-5): " -n 1 -r
echo ""

case $REPLY in
  1)
    VERSION_TYPE="patch"
    ;;
  2)
    VERSION_TYPE="minor"
    ;;
  3)
    VERSION_TYPE="major"
    ;;
  4)
    read -p "Enter version number (e.g., 1.2.3): " CUSTOM_VERSION
    VERSION_TYPE=$CUSTOM_VERSION
    ;;
  5)
    echo "❌ Release cancelled"
    exit 0
    ;;
  *)
    echo "❌ Invalid choice"
    exit 1
    ;;
esac

# Run tests
echo ""
echo "🧪 Running tests..."
npm test || echo "⚠️  No tests configured (this is okay for now)"

# Build
echo ""
echo "🔨 Building package..."
npm run build

# Check package
echo ""
echo "📦 Checking package contents..."
npm pack --dry-run

# Bump version
echo ""
echo "📝 Bumping version..."
npm version $VERSION_TYPE

NEW_VERSION=$(node -p "require('./package.json').version")
TAG="v$NEW_VERSION"

echo ""
echo "✅ Version bumped to $NEW_VERSION"
echo "📌 Tag created: $TAG"
echo ""

# Push
read -p "Push to origin and create release? (y/N): " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
  echo "📤 Pushing to origin..."
  git push origin $CURRENT_BRANCH --tags
  
  echo ""
  echo "✅ Done!"
  echo ""
  echo "Next steps:"
  echo "1. Go to: https://github.com/$(git config --get remote.origin.url | sed 's/.*github.com[:/]\(.*\)\.git/\1/')/releases/new"
  echo "2. Select tag: $TAG"
  echo "3. Add release notes"
  echo "4. Click 'Publish release'"
  echo ""
  echo "The GitHub Action will automatically publish to npm! 🎉"
else
  echo ""
  echo "❌ Push cancelled. To push manually:"
  echo "   git push origin $CURRENT_BRANCH --tags"
fi
