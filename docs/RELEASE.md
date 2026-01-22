# Release Process

This document describes how to publish a new version of `recursive-llm-ts` to npm.

## Prerequisites

### 1. NPM Token Setup (One-time)

You need to create an npm access token and add it to your GitHub repository secrets:

1. **Create npm token:**
   - Go to https://www.npmjs.com/
   - Log in to your account
   - Click your profile picture → "Access Tokens"
   - Click "Generate New Token" → "Classic Token"
   - Select "Automation" type
   - Copy the token

2. **Add token to GitHub:**
   - Go to your GitHub repository
   - Settings → Secrets and variables → Actions
   - Click "New repository secret"
   - Name: `NPM_TOKEN`
   - Value: Paste your npm token
   - Click "Add secret"

### 2. Package Name (One-time)

Make sure the package name in `package.json` is unique on npm:

```json
{
  "name": "recursive-llm-ts"
}
```

If the name is taken, choose a different name (e.g., `@yourorg/recursive-llm-ts` for scoped packages).

## Release Methods

### Method 1: Create a GitHub Release (Recommended)

This is the easiest way to publish:

1. **Update version in package.json:**
   ```bash
   npm version patch  # or minor, or major
   ```

2. **Push the tag:**
   ```bash
   git push origin main --tags
   ```

3. **Create GitHub Release:**
   - Go to your repository on GitHub
   - Click "Releases" → "Create a new release"
   - Click "Choose a tag" → Select the tag you just pushed (e.g., `v1.0.1`)
   - Fill in release title and description
   - Click "Publish release"

4. **Automatic Publish:**
   - GitHub Actions will automatically build and publish to npm
   - Check the "Actions" tab to monitor progress

### Method 2: Push a Version Tag

Skip the GitHub release UI and just push a tag:

1. **Update version:**
   ```bash
   npm version patch  # or minor, or major
   ```

2. **Push tag:**
   ```bash
   git push origin main --tags
   ```

3. **Automatic Publish:**
   - The workflow triggers on any `v*` tag
   - Package is automatically published to npm

## Version Bumping

Use npm's built-in version command:

```bash
# Patch release (1.0.0 -> 1.0.1) - bug fixes
npm version patch

# Minor release (1.0.0 -> 1.1.0) - new features, backward compatible
npm version minor

# Major release (1.0.0 -> 2.0.0) - breaking changes
npm version major
```

This automatically:
- Updates `package.json` version
- Creates a git commit
- Creates a git tag

## Manual Publishing (Not Recommended)

If you need to publish manually:

```bash
# Login to npm
npm login

# Build
npm run build

# Publish
npm publish --access public
```

## Workflow Details

### Publish Workflow

**Triggers:**
- Creating a GitHub release
- Pushing a tag starting with `v` (e.g., `v1.0.0`)

**Steps:**
1. Checkout code with submodules
2. Setup Node.js 20
3. Install dependencies
4. Build TypeScript
5. Publish to npm with provenance

**File:** `.github/workflows/publish.yml`

### CI Workflow

**Triggers:**
- Push to `main` or `develop` branches
- Pull requests to `main` or `develop`

**Tests:**
- Multiple Node.js versions (18, 20)
- Multiple OS (Ubuntu, macOS, Windows)

**File:** `.github/workflows/ci.yml`

## Pre-release Checklist

Before publishing a new version:

- [ ] Update version in `package.json`
- [ ] Update `CHANGELOG.md` (if you have one)
- [ ] Test the build: `npm run build`
- [ ] Check package contents: `npm pack --dry-run`
- [ ] Test installation in another project: `npm pack` → `npm install ./recursive-llm-ts-1.0.0.tgz`
- [ ] Commit all changes
- [ ] Create and push version tag
- [ ] Create GitHub release (optional but recommended)

## Troubleshooting

### "You must be logged in to publish packages"

Make sure the `NPM_TOKEN` secret is set correctly in GitHub repository settings.

### "You cannot publish over the previously published versions"

The version in `package.json` already exists on npm. Bump the version number.

### "Package name too similar to existing package"

Choose a different package name or use a scoped package (e.g., `@yourorg/recursive-llm-ts`).

### Build Fails

Check the GitHub Actions logs:
1. Go to "Actions" tab in your repository
2. Click on the failed workflow
3. Check the logs for errors

## Rolling Back

If you need to deprecate or unpublish a version:

```bash
# Deprecate a version (recommended over unpublish)
npm deprecate recursive-llm-ts@1.0.0 "Use version 1.0.1 instead"

# Unpublish (only within 72 hours of publishing)
npm unpublish recursive-llm-ts@1.0.0
```

⚠️ **Note:** Unpublishing is discouraged as it can break projects depending on that version.
