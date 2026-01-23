# Changelog

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
