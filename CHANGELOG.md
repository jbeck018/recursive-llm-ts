# Changelog

## [4.4.0] - 2026-02-09

### ✨ New Features
- **File Storage Context** - Process local directories or S3 buckets as LLM context
  - `completionFromFiles()` and `structuredCompletionFromFiles()` methods on RLM class
  - `FileContextBuilder` for building structured context from file trees
  - `LocalFileStorage` for local filesystem traversal with recursive directory support
  - `S3FileStorage` for AWS S3, MinIO, LocalStack, DigitalOcean Spaces, Backblaze B2
  - Configurable filters: extensions, glob include/exclude patterns, max file/total size, max files
  - Per-file error recovery (failed reads are skipped, not fatal)
- **S3 Credential Chain** - Flexible credential resolution for S3 storage
  - Explicit credentials > environment variables > AWS SDK default chain
  - Environment variable support: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`
  - `AWS_DEFAULT_REGION` fallback alongside `AWS_REGION`
- **S3StorageError** - Typed error class with actionable messages
  - Error codes: `AUTH_FAILED`, `ACCESS_DENIED`, `BUCKET_NOT_FOUND`, `KEY_NOT_FOUND`, `NETWORK_ERROR`, `REGION_MISMATCH`
  - User-friendly remediation hints for each error type
- **S3-Compatible Services** - First-class support for non-AWS providers
  - Custom endpoint configuration with automatic `forcePathStyle`
  - Tested with MinIO, LocalStack, DigitalOcean Spaces, Backblaze B2

### 🔧 Fixed
- **Parallel structured output** - Rewrote JSON extraction with balanced-brace parser (replaces shallow regex)
  - Per-field schema wrapping eliminates ambiguity for non-object types
  - Automatic fallback from parallel to direct single-call on failure
  - Better error collection captures all parallel failures
- **Type exports** - Added missing exports: `SubTask`, `CoordinatorConfig`, `SchemaDecomposition`, `S3StorageError`
- **FileStorageConfig** - Consolidated to single canonical definition (was duplicated)
- **Debug leak** - Removed leftover `console.error('[RLM_DEBUG]...')` from production code

### 🧪 Testing
- 117 file storage tests: local operations, S3 construction, credential resolution, region resolution, error wrapping, endpoint configs, mock error scenarios, error recovery, forcePathStyle
- 44 Go tests for structured output: balanced JSON extraction, schema wrapping, validation, parallel merging

## [4.3.0] - 2026-01-23

### ✨ Major Improvements
- **Instructor-style retry logic**: Enhanced validation feedback with detailed error messages showing exactly what's missing
- **100% reliability**: Parallel execution now passes all reliability tests with nested objects
- **Comprehensive JSON Schema support**: Added support for all major constraint types

### 🔧 Fixed
- **Retry loop**: Now properly accumulates conversation history including previous attempts and validation feedback
- **Nested object extraction**: LLM now reliably returns all required fields in nested objects during parallel execution

### 📦 Added JSON Schema Support
- Number constraints: `minimum`, `maximum`, `multipleOf`
- String constraints: `minLength`, `maxLength`, `pattern`, `format`
- Array constraints: `minItems`, `maxItems`, `uniqueItems`
- Object constraints: `additionalProperties`
- Union/Intersection: `anyOf`, `allOf`

### 🎯 Enhanced Features
- **Example JSON generation**: Automatically generates example structures for nested objects
- **Detailed constraint messages**: Shows actual min/max values from schema in prompts
- **Better validation feedback**: Provides specific guidance on what fields are missing and their types

### 🧪 Testing
- Added comprehensive reliability test suite (`examples/test-reliability.ts`)
- Tests run 5 consecutive parallel extractions with 100% success rate
- Validates nested object extraction with required fields

## [4.2.0] - 2026-01-23

### Added
- Initial validation feedback in retry logic
- Basic constraint generation for nested objects

## [4.1.0] - Previous

### Features
- Zod schema to JSON Schema conversion
- Parallel execution for large schemas
- Recursive extraction service integration
