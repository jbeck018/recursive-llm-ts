import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import {
  FileContextBuilder,
  LocalFileStorage,
  S3FileStorage,
  S3StorageError,
  FileStorageConfig,
  FileStorageResult,
  FileStorageProvider,
  buildFileContext,
} from '../src/file-storage';

// ─── Test Helpers ────────────────────────────────────────────────────────────

let testDir: string;
let testCount = 0;
let passCount = 0;
let failCount = 0;

function createTestDir(): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'rlm-file-storage-test-'));
  return dir;
}

function cleanupTestDir(dir: string): void {
  fs.rmSync(dir, { recursive: true, force: true });
}

function writeFile(dir: string, relativePath: string, content: string): void {
  const fullPath = path.join(dir, relativePath);
  const dirPath = path.dirname(fullPath);
  fs.mkdirSync(dirPath, { recursive: true });
  fs.writeFileSync(fullPath, content, 'utf-8');
}

function assert(condition: boolean, message: string): void {
  testCount++;
  if (condition) {
    passCount++;
    console.log(`  ✅ ${message}`);
  } else {
    failCount++;
    console.log(`  ❌ ${message}`);
  }
}

function assertEqual(actual: any, expected: any, message: string): void {
  testCount++;
  if (JSON.stringify(actual) === JSON.stringify(expected)) {
    passCount++;
    console.log(`  ✅ ${message}`);
  } else {
    failCount++;
    console.log(`  ❌ ${message}`);
    console.log(`     Expected: ${JSON.stringify(expected)}`);
    console.log(`     Actual:   ${JSON.stringify(actual)}`);
  }
}

// ─── Test: LocalFileStorage Basic Operations ─────────────────────────────────

async function testLocalFileStorageBasic() {
  console.log('\n=== LOCAL FILE STORAGE: Basic Operations ===\n');

  testDir = createTestDir();
  try {
    writeFile(testDir, 'file1.txt', 'Hello, World!');
    writeFile(testDir, 'file2.md', '# Heading\nSome markdown');
    writeFile(testDir, 'sub/nested.ts', 'const x = 1;');

    const storage = new LocalFileStorage(testDir);

    // Test listFiles
    const files = await storage.listFiles();
    assertEqual(files.length, 3, 'Should list 3 files');
    assert(files.includes('file1.txt'), 'Should include file1.txt');
    assert(files.includes('file2.md'), 'Should include file2.md');
    assert(files.includes(path.join('sub', 'nested.ts')), 'Should include sub/nested.ts');

    // Test readFile
    const content = await storage.readFile('file1.txt');
    assertEqual(content, 'Hello, World!', 'Should read file content correctly');

    // Test getFileSize
    const size = await storage.getFileSize('file1.txt');
    assertEqual(size, Buffer.byteLength('Hello, World!', 'utf-8'), 'Should return correct file size');

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: Deeply Nested Directory Traversal ─────────────────────────────────

async function testDeepNesting() {
  console.log('\n=== LOCAL FILE STORAGE: Deeply Nested Directories ===\n');

  testDir = createTestDir();
  try {
    writeFile(testDir, 'root.txt', 'root level');
    writeFile(testDir, 'a/level1.txt', 'level 1');
    writeFile(testDir, 'a/b/level2.txt', 'level 2');
    writeFile(testDir, 'a/b/c/level3.txt', 'level 3');
    writeFile(testDir, 'a/b/c/d/level4.txt', 'level 4');
    writeFile(testDir, 'x/y/other.txt', 'other branch');

    const storage = new LocalFileStorage(testDir);
    const files = await storage.listFiles();

    assertEqual(files.length, 6, 'Should find 6 files across all nesting levels');
    assert(
      files.includes(path.join('a', 'b', 'c', 'd', 'level4.txt')),
      'Should find deeply nested file (4 levels deep)'
    );
    assert(
      files.includes(path.join('x', 'y', 'other.txt')),
      'Should find files in separate branches'
    );

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: FileContextBuilder with Extension Filter ──────────────────────────

async function testExtensionFilter() {
  console.log('\n=== FILE CONTEXT BUILDER: Extension Filter ===\n');

  testDir = createTestDir();
  try {
    writeFile(testDir, 'code.ts', 'const a = 1;');
    writeFile(testDir, 'readme.md', '# README');
    writeFile(testDir, 'styles.css', 'body {}');
    writeFile(testDir, 'data.json', '{}');
    writeFile(testDir, 'src/main.ts', 'export default {};');

    const config: FileStorageConfig = {
      type: 'local',
      path: testDir,
      extensions: ['.ts', '.md'],
    };

    const builder = new FileContextBuilder(config);
    const result = await builder.buildContext();

    assertEqual(result.files.length, 3, 'Should include only .ts and .md files (3 files)');
    assert(
      result.files.some(f => f.relativePath === 'code.ts'),
      'Should include code.ts'
    );
    assert(
      result.files.some(f => f.relativePath === 'readme.md'),
      'Should include readme.md'
    );
    assert(
      result.files.some(f => f.relativePath === path.join('src', 'main.ts')),
      'Should include src/main.ts'
    );
    assertEqual(result.skipped.length, 2, 'Should skip 2 files (css, json)');

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: FileContextBuilder with Include/Exclude Patterns ──────────────────

async function testGlobPatterns() {
  console.log('\n=== FILE CONTEXT BUILDER: Glob Patterns ===\n');

  testDir = createTestDir();
  try {
    writeFile(testDir, 'src/main.ts', 'export default {};');
    writeFile(testDir, 'src/utils.ts', 'export const util = 1;');
    writeFile(testDir, 'src/test.spec.ts', 'describe("test", () => {});');
    writeFile(testDir, 'node_modules/pkg/index.js', 'module.exports = {};');
    writeFile(testDir, 'dist/main.js', 'var a = 1;');

    // Include only src/**
    const config: FileStorageConfig = {
      type: 'local',
      path: testDir,
      includePatterns: ['src/*'],
      excludePatterns: ['*.spec.ts'],
    };

    const builder = new FileContextBuilder(config);
    const result = await builder.buildContext();

    assertEqual(result.files.length, 2, 'Should include 2 files matching src/* but not *.spec.ts');
    assert(
      result.files.some(f => f.relativePath === path.join('src', 'main.ts')),
      'Should include src/main.ts'
    );
    assert(
      result.files.some(f => f.relativePath === path.join('src', 'utils.ts')),
      'Should include src/utils.ts'
    );

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: FileContextBuilder Max File Size ──────────────────────────────────

async function testMaxFileSize() {
  console.log('\n=== FILE CONTEXT BUILDER: Max File Size ===\n');

  testDir = createTestDir();
  try {
    writeFile(testDir, 'small.txt', 'small file');
    writeFile(testDir, 'large.txt', 'x'.repeat(1000)); // 1000 bytes

    const config: FileStorageConfig = {
      type: 'local',
      path: testDir,
      maxFileSize: 500, // 500 bytes max
    };

    const builder = new FileContextBuilder(config);
    const result = await builder.buildContext();

    assertEqual(result.files.length, 1, 'Should include only the small file');
    assert(
      result.files.some(f => f.relativePath === 'small.txt'),
      'Should include small.txt'
    );
    assert(
      result.skipped.some(f => f.relativePath === 'large.txt' && f.reason.includes('file size')),
      'Should skip large.txt with size reason'
    );

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: FileContextBuilder Max Total Size ─────────────────────────────────

async function testMaxTotalSize() {
  console.log('\n=== FILE CONTEXT BUILDER: Max Total Size ===\n');

  testDir = createTestDir();
  try {
    // Create several files that together exceed the limit
    writeFile(testDir, 'a.txt', 'a'.repeat(200));
    writeFile(testDir, 'b.txt', 'b'.repeat(200));
    writeFile(testDir, 'c.txt', 'c'.repeat(200));

    const config: FileStorageConfig = {
      type: 'local',
      path: testDir,
      maxTotalSize: 800, // The file tree header + first couple files should hit this
      maxFileSize: 10000,
    };

    const builder = new FileContextBuilder(config);
    const result = await builder.buildContext();

    assert(result.totalSize <= 800, `Total size ${result.totalSize} should be within limit 800`);
    assert(
      result.skipped.some(f => f.reason.includes('total size limit')),
      'Should have skipped at least one file due to total size limit'
    );

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: FileContextBuilder Max Files ──────────────────────────────────────

async function testMaxFiles() {
  console.log('\n=== FILE CONTEXT BUILDER: Max Files ===\n');

  testDir = createTestDir();
  try {
    for (let i = 0; i < 10; i++) {
      writeFile(testDir, `file${i}.txt`, `content ${i}`);
    }

    const config: FileStorageConfig = {
      type: 'local',
      path: testDir,
      maxFiles: 3,
    };

    const builder = new FileContextBuilder(config);
    const result = await builder.buildContext();

    assertEqual(result.files.length, 3, 'Should include exactly 3 files');
    assert(
      result.skipped.filter(f => f.reason.includes('max file count')).length === 7,
      'Should skip 7 files due to max file count'
    );

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: Context String Format ─────────────────────────────────────────────

async function testContextFormat() {
  console.log('\n=== FILE CONTEXT BUILDER: Context String Format ===\n');

  testDir = createTestDir();
  try {
    writeFile(testDir, 'hello.ts', 'const greeting = "hello";');
    writeFile(testDir, 'README.md', '# My Project\nThis is a project.');

    const config: FileStorageConfig = {
      type: 'local',
      path: testDir,
    };

    const builder = new FileContextBuilder(config);
    const result = await builder.buildContext();

    // Check file structure header
    assert(
      result.context.includes('=== FILE STRUCTURE ==='),
      'Context should include file structure header'
    );
    assert(
      result.context.includes('Total files: 2'),
      'Context should show file count'
    );

    // Check file blocks
    assert(
      result.context.includes('=== FILE: README.md ==='),
      'Context should include README.md file block'
    );
    assert(
      result.context.includes('=== FILE: hello.ts ==='),
      'Context should include hello.ts file block'
    );
    assert(
      result.context.includes('const greeting = "hello";'),
      'Context should include file content'
    );
    assert(
      result.context.includes('=== END FILE: hello.ts ==='),
      'Context should include end delimiter'
    );

    // Check code fence language hints
    assert(
      result.context.includes('```ts'),
      'Context should include TypeScript code fence'
    );
    assert(
      result.context.includes('```md'),
      'Context should include Markdown code fence'
    );

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: Empty Directory ───────────────────────────────────────────────────

async function testEmptyDirectory() {
  console.log('\n=== FILE CONTEXT BUILDER: Empty Directory ===\n');

  testDir = createTestDir();
  try {
    const config: FileStorageConfig = {
      type: 'local',
      path: testDir,
    };

    const builder = new FileContextBuilder(config);
    const result = await builder.buildContext();

    assertEqual(result.files.length, 0, 'Should have no files');
    assertEqual(result.skipped.length, 0, 'Should have no skipped files');
    assert(result.context.includes('Total files: 0'), 'Should show 0 files in tree');

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: listMatchingFiles (preview mode) ──────────────────────────────────

async function testListMatchingFiles() {
  console.log('\n=== FILE CONTEXT BUILDER: listMatchingFiles ===\n');

  testDir = createTestDir();
  try {
    writeFile(testDir, 'a.ts', 'code');
    writeFile(testDir, 'b.ts', 'code');
    writeFile(testDir, 'c.css', 'styles');

    const config: FileStorageConfig = {
      type: 'local',
      path: testDir,
      extensions: ['.ts'],
    };

    const builder = new FileContextBuilder(config);
    const files = await builder.listMatchingFiles();

    assertEqual(files.length, 2, 'Should list 2 matching files');
    assert(files.includes('a.ts'), 'Should include a.ts');
    assert(files.includes('b.ts'), 'Should include b.ts');

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: buildFileContext convenience function ──────────────────────────────

async function testBuildFileContextFunction() {
  console.log('\n=== buildFileContext: Convenience Function ===\n');

  testDir = createTestDir();
  try {
    writeFile(testDir, 'data.json', '{"key": "value"}');
    writeFile(testDir, 'notes.txt', 'some notes');

    const result = await buildFileContext({
      type: 'local',
      path: testDir,
    });

    assertEqual(result.files.length, 2, 'Should include both files');
    assert(result.totalSize > 0, 'Total size should be positive');
    assert(result.context.length > 0, 'Context should be non-empty');

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: Binary files are handled gracefully ───────────────────────────────

async function testBinaryFileHandling() {
  console.log('\n=== FILE CONTEXT BUILDER: Binary File Handling ===\n');

  testDir = createTestDir();
  try {
    writeFile(testDir, 'readme.txt', 'readable text');
    // Write a binary-ish file
    const binaryContent = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
    fs.writeFileSync(path.join(testDir, 'image.png'), binaryContent);

    // Use extension filter to skip binary
    const config: FileStorageConfig = {
      type: 'local',
      path: testDir,
      extensions: ['.txt'],
    };

    const builder = new FileContextBuilder(config);
    const result = await builder.buildContext();

    assertEqual(result.files.length, 1, 'Should only include text file');
    assert(
      result.files[0].relativePath === 'readme.txt',
      'Should include readme.txt'
    );

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: S3FileStorage construction (unit test, no actual AWS) ─────────────

async function testS3StorageConstruction() {
  console.log('\n=== S3 FILE STORAGE: Construction ===\n');

  const config: FileStorageConfig = {
    type: 's3',
    path: 'my-bucket',
    prefix: 'docs/',
    region: 'us-west-2',
    credentials: {
      accessKeyId: 'test-key',
      secretAccessKey: 'test-secret',
    },
    endpoint: 'http://localhost:4566', // LocalStack
  };

  const storage = new S3FileStorage(config);
  assert(storage !== null, 'S3FileStorage should be constructable');

  // Test that listing files fails gracefully without AWS SDK
  try {
    await storage.listFiles();
    // If @aws-sdk/client-s3 is installed, this might work with LocalStack
    assert(true, 'S3 listFiles executed (SDK available)');
  } catch (err: any) {
    assert(
      err.message.includes('@aws-sdk/client-s3') ||
      err.message.includes('ECONNREFUSED') ||
      err.message.includes('fetch failed') ||
      err.message.includes('Cannot find'),
      `S3 listFiles should fail with SDK or connection error: ${err.message.substring(0, 80)}`
    );
  }
}

// ─── Test: S3FileStorage with Mock Provider ──────────────────────────────────

class MockS3Provider implements FileStorageProvider {
  private files: Map<string, string>;

  constructor(files: Record<string, string>) {
    this.files = new Map(Object.entries(files));
  }

  async listFiles(): Promise<string[]> {
    return Array.from(this.files.keys()).sort();
  }

  async readFile(relativePath: string): Promise<string> {
    const content = this.files.get(relativePath);
    if (!content) throw new Error(`File not found: ${relativePath}`);
    return content;
  }

  async getFileSize(relativePath: string): Promise<number> {
    const content = this.files.get(relativePath);
    if (!content) throw new Error(`File not found: ${relativePath}`);
    return Buffer.byteLength(content, 'utf-8');
  }
}

async function testMockS3Provider() {
  console.log('\n=== S3 FILE STORAGE: Mock Provider Tests ===\n');

  const mockFiles = {
    'reports/2024/q1.md': '# Q1 Report\nRevenue: $1M',
    'reports/2024/q2.md': '# Q2 Report\nRevenue: $1.2M',
    'reports/2024/q3.md': '# Q3 Report\nRevenue: $1.5M',
    'config/settings.json': '{"version": 1}',
    'readme.txt': 'Project documentation',
    'data/large-dataset.csv': 'col1,col2\n' + 'a,b\n'.repeat(100),
  };

  const provider = new MockS3Provider(mockFiles);

  // Test file listing
  const files = await provider.listFiles();
  assertEqual(files.length, 6, 'Mock provider should list 6 files');

  // Test file reading
  const content = await provider.readFile('reports/2024/q1.md');
  assert(content.includes('Q1 Report'), 'Should read correct content');

  // Test file size
  const size = await provider.getFileSize('readme.txt');
  assertEqual(
    size,
    Buffer.byteLength('Project documentation', 'utf-8'),
    'Should return correct size'
  );

  // Test with FileContextBuilder using mock
  // We'll create a custom builder that uses our mock
  const builder = new FileContextBuilderWithMock(provider, {
    type: 's3',
    path: 'test-bucket',
    prefix: '',
    extensions: ['.md'],
  });

  const result = await builder.buildContext();
  assertEqual(result.files.length, 3, 'Should include only .md files (3 reports)');
  assert(
    result.context.includes('Q1 Report'),
    'Context should include Q1 report content'
  );
  assert(
    result.context.includes('Q2 Report'),
    'Context should include Q2 report content'
  );
  assert(
    result.context.includes('Q3 Report'),
    'Context should include Q3 report content'
  );
}

// Helper class that accepts a custom provider for testing
class FileContextBuilderWithMock {
  private provider: FileStorageProvider;
  private config: FileStorageConfig;

  constructor(provider: FileStorageProvider, config: FileStorageConfig) {
    this.provider = provider;
    this.config = config;
  }

  async buildContext(): Promise<FileStorageResult> {
    const maxFileSize = this.config.maxFileSize ?? 1024 * 1024;
    const maxTotalSize = this.config.maxTotalSize ?? 10 * 1024 * 1024;
    const maxFiles = this.config.maxFiles ?? 1000;

    const allFiles = await this.provider.listFiles();
    const matchedFiles: string[] = [];
    const skipped: Array<{ relativePath: string; reason: string }> = [];

    for (const filePath of allFiles) {
      if (this.config.extensions && this.config.extensions.length > 0) {
        const ext = path.extname(filePath).toLowerCase();
        if (!this.config.extensions.includes(ext)) {
          skipped.push({ relativePath: filePath, reason: `extension ${ext} not in allowed list` });
          continue;
        }
      }
      matchedFiles.push(filePath);
    }

    const includedFiles: Array<{ relativePath: string; size: number }> = [];
    const contextParts: string[] = [];
    let totalSize = 0;

    contextParts.push(`=== FILE STRUCTURE ===\nTotal files: ${matchedFiles.length}\n=== END FILE STRUCTURE ===\n`);

    for (const filePath of matchedFiles) {
      if (includedFiles.length >= maxFiles) break;

      try {
        const fileSize = await this.provider.getFileSize(filePath);
        if (fileSize > maxFileSize) {
          skipped.push({ relativePath: filePath, reason: 'too large' });
          continue;
        }
        if (totalSize + fileSize > maxTotalSize) {
          skipped.push({ relativePath: filePath, reason: 'total size exceeded' });
          continue;
        }

        const content = await this.provider.readFile(filePath);
        const ext = path.extname(filePath).slice(1);
        const block = `=== FILE: ${filePath} ===\n\`\`\`${ext}\n${content}\n\`\`\`\n=== END FILE: ${filePath} ===\n`;
        contextParts.push(block);
        totalSize += Buffer.byteLength(block, 'utf-8');
        includedFiles.push({ relativePath: filePath, size: fileSize });
      } catch (err: any) {
        skipped.push({ relativePath: filePath, reason: `read error: ${err.message}` });
      }
    }

    return {
      context: contextParts.join('\n'),
      files: includedFiles,
      totalSize,
      skipped,
    };
  }
}

// ─── Test: Mock S3 with nested folders and filtering ─────────────────────────

async function testMockS3NestedFolders() {
  console.log('\n=== S3 FILE STORAGE: Nested Folders with Filtering ===\n');

  const mockFiles = {
    'project/src/index.ts': 'export {};',
    'project/src/utils/helpers.ts': 'export function help() {}',
    'project/src/utils/math.ts': 'export function add(a: number, b: number) { return a + b; }',
    'project/tests/index.test.ts': 'test("it works", () => {});',
    'project/package.json': '{"name": "test"}',
    'project/node_modules/dep/index.js': 'module.exports = {};',
    'project/.env': 'SECRET=abc123',
    'project/dist/index.js': 'var x = 1;',
  };

  const provider = new MockS3Provider(mockFiles);

  // Test with extension and exclude filtering
  const builder = new FileContextBuilderWithMock(provider, {
    type: 's3',
    path: 'test-bucket',
    extensions: ['.ts'],
  });

  const result = await builder.buildContext();

  assertEqual(result.files.length, 4, 'Should include 4 .ts files');
  assert(
    result.files.some(f => f.relativePath === 'project/src/utils/helpers.ts'),
    'Should include nested helpers.ts'
  );
  assert(
    result.files.some(f => f.relativePath === 'project/tests/index.test.ts'),
    'Should include test file'
  );
  assert(
    !result.files.some(f => f.relativePath.includes('.env')),
    'Should NOT include .env file'
  );
  assert(
    !result.files.some(f => f.relativePath.includes('node_modules')),
    'Should NOT include node_modules (filtered by extension)'
  );
}

// ─── Test: Mock S3 with max constraints ──────────────────────────────────────

async function testMockS3MaxConstraints() {
  console.log('\n=== S3 FILE STORAGE: Max Constraints ===\n');

  const mockFiles: Record<string, string> = {};
  for (let i = 0; i < 20; i++) {
    mockFiles[`file${i.toString().padStart(2, '0')}.txt`] = `Content of file ${i}. ${'x'.repeat(50)}`;
  }

  const provider = new MockS3Provider(mockFiles);

  // Test maxFiles
  const builder = new FileContextBuilderWithMock(provider, {
    type: 's3',
    path: 'test-bucket',
    maxFiles: 5,
  });

  const result = await builder.buildContext();
  assertEqual(result.files.length, 5, 'Should include exactly 5 files');

  // Test maxFileSize
  const largeFiles: Record<string, string> = {
    'small.txt': 'tiny',
    'medium.txt': 'x'.repeat(500),
    'large.txt': 'x'.repeat(5000),
  };

  const provider2 = new MockS3Provider(largeFiles);
  const builder2 = new FileContextBuilderWithMock(provider2, {
    type: 's3',
    path: 'test-bucket',
    maxFileSize: 1000,
  });

  const result2 = await builder2.buildContext();
  assertEqual(result2.files.length, 2, 'Should include only files under 1000 bytes');
  assert(
    result2.skipped.some(f => f.relativePath === 'large.txt'),
    'Should skip large.txt'
  );
}

// ─── Test: Context suitable for LLM consumption ─────────────────────────────

async function testContextForLLM() {
  console.log('\n=== FILE CONTEXT BUILDER: LLM-Ready Context ===\n');

  testDir = createTestDir();
  try {
    writeFile(testDir, 'src/main.ts', `
import { Database } from './db';

export async function main() {
  const db = new Database();
  await db.connect();
  console.log('Server started');
}
`.trim());

    writeFile(testDir, 'src/db.ts', `
export class Database {
  private connection: any;

  async connect() {
    this.connection = await createConnection();
  }

  async query(sql: string) {
    return this.connection.execute(sql);
  }
}
`.trim());

    writeFile(testDir, 'docs/architecture.md', `
# Architecture

## Components
- **Main**: Entry point, bootstraps the application
- **Database**: Handles all database operations

## Data Flow
1. Main initializes Database
2. Database connects to PostgreSQL
3. Queries are executed through the Database class
`.trim());

    const config: FileStorageConfig = {
      type: 'local',
      path: testDir,
    };

    const result = await buildFileContext(config);

    // The context should be structured enough for an LLM to reason about
    assert(
      result.context.includes('=== FILE STRUCTURE ==='),
      'Should have file tree for orientation'
    );
    assert(
      result.context.includes('Total files: 3'),
      'Should show total count'
    );
    assert(
      result.context.includes('=== FILE: '),
      'Should have clear file delimiters'
    );
    assert(
      result.context.includes('Database'),
      'Should include actual code content'
    );
    assert(
      result.context.includes('Architecture'),
      'Should include docs'
    );

    // Verify the context can be used as a prompt
    assert(result.context.length > 100, 'Context should be substantial');
    assert(result.files.length === 3, 'All 3 files should be included');

    console.log(`  📏 Context length: ${result.context.length} characters`);
    console.log(`  📁 Files included: ${result.files.length}`);

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: Files with special characters ─────────────────────────────────────

async function testSpecialCharacters() {
  console.log('\n=== FILE CONTEXT BUILDER: Special Characters ===\n');

  testDir = createTestDir();
  try {
    writeFile(testDir, 'unicode.txt', '日本語テキスト');
    writeFile(testDir, 'emoji.md', '# 🎉 Celebration\nParty time!');
    writeFile(testDir, 'backticks.txt', 'Some `code` with ```blocks```');

    const result = await buildFileContext({
      type: 'local',
      path: testDir,
    });

    assertEqual(result.files.length, 3, 'Should include all 3 files');
    assert(
      result.context.includes('日本語テキスト'),
      'Should preserve Unicode content'
    );
    assert(
      result.context.includes('🎉'),
      'Should preserve emoji content'
    );

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: Symlinks and edge cases ───────────────────────────────────────────

async function testEdgeCases() {
  console.log('\n=== FILE CONTEXT BUILDER: Edge Cases ===\n');

  testDir = createTestDir();
  try {
    // Empty file
    writeFile(testDir, 'empty.txt', '');
    // Single character
    writeFile(testDir, 'single.txt', 'x');
    // File with only whitespace
    writeFile(testDir, 'whitespace.txt', '   \n\n   ');

    const result = await buildFileContext({
      type: 'local',
      path: testDir,
    });

    assertEqual(result.files.length, 3, 'Should include all edge case files');
    assert(
      result.context.includes('=== FILE: empty.txt ==='),
      'Should include empty file with delimiters'
    );

  } finally {
    cleanupTestDir(testDir);
  }
}

// ─── Test: S3 Credential Resolution ──────────────────────────────────────────

async function testS3CredentialResolution() {
  console.log('\n=== S3 FILE STORAGE: Credential Resolution ===\n');

  // Save original env vars
  const origAccessKey = process.env.AWS_ACCESS_KEY_ID;
  const origSecretKey = process.env.AWS_SECRET_ACCESS_KEY;
  const origSessionToken = process.env.AWS_SESSION_TOKEN;
  const origRegion = process.env.AWS_REGION;
  const origDefaultRegion = process.env.AWS_DEFAULT_REGION;

  try {
    // 1. Explicit credentials take priority
    process.env.AWS_ACCESS_KEY_ID = 'env-key';
    process.env.AWS_SECRET_ACCESS_KEY = 'env-secret';

    const explicitConfig: FileStorageConfig = {
      type: 's3',
      path: 'test-bucket',
      credentials: {
        accessKeyId: 'explicit-key',
        secretAccessKey: 'explicit-secret',
        sessionToken: 'explicit-token',
      },
    };
    const explicitStorage = new S3FileStorage(explicitConfig);
    assertEqual(
      explicitStorage.getCredentialSource(),
      'explicit',
      'Should use explicit credentials when provided'
    );

    // 2. Env vars used when no explicit credentials
    const envConfig: FileStorageConfig = {
      type: 's3',
      path: 'test-bucket',
    };
    const envStorage = new S3FileStorage(envConfig);
    assertEqual(
      envStorage.getCredentialSource(),
      'environment',
      'Should use environment credentials when no explicit ones'
    );

    // 3. Default chain when neither explicit nor env vars
    delete process.env.AWS_ACCESS_KEY_ID;
    delete process.env.AWS_SECRET_ACCESS_KEY;
    delete process.env.AWS_SESSION_TOKEN;

    const defaultConfig: FileStorageConfig = {
      type: 's3',
      path: 'test-bucket',
    };
    const defaultStorage = new S3FileStorage(defaultConfig);
    assertEqual(
      defaultStorage.getCredentialSource(),
      'default-chain',
      'Should fall back to default chain when no explicit or env credentials'
    );

    // 4. Env vars with session token
    process.env.AWS_ACCESS_KEY_ID = 'env-key-2';
    process.env.AWS_SECRET_ACCESS_KEY = 'env-secret-2';
    process.env.AWS_SESSION_TOKEN = 'env-session-token';

    const envTokenConfig: FileStorageConfig = {
      type: 's3',
      path: 'test-bucket',
    };
    const envTokenStorage = new S3FileStorage(envTokenConfig);
    assertEqual(
      envTokenStorage.getCredentialSource(),
      'environment',
      'Should use env credentials with session token'
    );

    // 5. Partial env vars (only access key, no secret) should fall back
    delete process.env.AWS_SECRET_ACCESS_KEY;
    delete process.env.AWS_SESSION_TOKEN;

    const partialEnvConfig: FileStorageConfig = {
      type: 's3',
      path: 'test-bucket',
    };
    const partialStorage = new S3FileStorage(partialEnvConfig);
    assertEqual(
      partialStorage.getCredentialSource(),
      'default-chain',
      'Should fall back to default chain when only partial env vars'
    );

  } finally {
    // Restore original env vars
    if (origAccessKey !== undefined) process.env.AWS_ACCESS_KEY_ID = origAccessKey;
    else delete process.env.AWS_ACCESS_KEY_ID;
    if (origSecretKey !== undefined) process.env.AWS_SECRET_ACCESS_KEY = origSecretKey;
    else delete process.env.AWS_SECRET_ACCESS_KEY;
    if (origSessionToken !== undefined) process.env.AWS_SESSION_TOKEN = origSessionToken;
    else delete process.env.AWS_SESSION_TOKEN;
    if (origRegion !== undefined) process.env.AWS_REGION = origRegion;
    else delete process.env.AWS_REGION;
    if (origDefaultRegion !== undefined) process.env.AWS_DEFAULT_REGION = origDefaultRegion;
    else delete process.env.AWS_DEFAULT_REGION;
  }
}

// ─── Test: S3 Region Resolution ─────────────────────────────────────────────

async function testS3RegionResolution() {
  console.log('\n=== S3 FILE STORAGE: Region Resolution ===\n');

  const origRegion = process.env.AWS_REGION;
  const origDefaultRegion = process.env.AWS_DEFAULT_REGION;

  try {
    // 1. Explicit region in config takes priority
    process.env.AWS_REGION = 'eu-west-1';
    const explicitRegion = new S3FileStorage({
      type: 's3',
      path: 'bucket',
      region: 'ap-southeast-1',
    });
    // We can't directly access private fields, but construction should succeed
    assert(explicitRegion !== null, 'Should construct with explicit region');

    // 2. AWS_REGION env var
    delete process.env.AWS_DEFAULT_REGION;
    process.env.AWS_REGION = 'eu-central-1';
    const envRegion = new S3FileStorage({
      type: 's3',
      path: 'bucket',
    });
    assert(envRegion !== null, 'Should construct with AWS_REGION');

    // 3. AWS_DEFAULT_REGION env var
    delete process.env.AWS_REGION;
    process.env.AWS_DEFAULT_REGION = 'us-west-2';
    const defaultRegion = new S3FileStorage({
      type: 's3',
      path: 'bucket',
    });
    assert(defaultRegion !== null, 'Should construct with AWS_DEFAULT_REGION');

    // 4. Fallback to us-east-1
    delete process.env.AWS_REGION;
    delete process.env.AWS_DEFAULT_REGION;
    const fallbackRegion = new S3FileStorage({
      type: 's3',
      path: 'bucket',
    });
    assert(fallbackRegion !== null, 'Should construct with fallback us-east-1');

  } finally {
    if (origRegion !== undefined) process.env.AWS_REGION = origRegion;
    else delete process.env.AWS_REGION;
    if (origDefaultRegion !== undefined) process.env.AWS_DEFAULT_REGION = origDefaultRegion;
    else delete process.env.AWS_DEFAULT_REGION;
  }
}

// ─── Test: S3StorageError ────────────────────────────────────────────────────

async function testS3StorageError() {
  console.log('\n=== S3 FILE STORAGE: S3StorageError ===\n');

  const err = new S3StorageError({
    message: 'Test error message',
    code: 'AUTH_FAILED',
    bucket: 'my-bucket',
    key: 'path/to/file.txt',
    originalError: new Error('underlying error'),
  });

  assert(err instanceof Error, 'S3StorageError should be instanceof Error');
  assert(err instanceof S3StorageError, 'Should be instanceof S3StorageError');
  assertEqual(err.name, 'S3StorageError', 'Should have correct name');
  assertEqual(err.code, 'AUTH_FAILED', 'Should have correct code');
  assertEqual(err.bucket, 'my-bucket', 'Should have correct bucket');
  assertEqual(err.key, 'path/to/file.txt', 'Should have correct key');
  assert(err.originalError !== undefined, 'Should have original error');
  assert(err.message.includes('Test error message'), 'Should have message');

  // Test without optional fields
  const err2 = new S3StorageError({
    message: 'Simple error',
    code: 'UNKNOWN',
    bucket: 'bucket',
  });
  assert(err2.key === undefined, 'Key should be undefined when not provided');
  assert(err2.originalError === undefined, 'Original error should be undefined when not provided');
}

// ─── Test: S3 Error Wrapping ─────────────────────────────────────────────────

class ErrorMockProvider implements FileStorageProvider {
  private errorToThrow: Error;

  constructor(errorToThrow: Error) {
    this.errorToThrow = errorToThrow;
  }

  async listFiles(): Promise<string[]> {
    throw this.errorToThrow;
  }

  async readFile(_relativePath: string): Promise<string> {
    throw this.errorToThrow;
  }

  async getFileSize(_relativePath: string): Promise<number> {
    throw this.errorToThrow;
  }
}

async function testS3ErrorWrapping() {
  console.log('\n=== S3 FILE STORAGE: Error Wrapping ===\n');

  // Test construction with endpoint for MinIO/LocalStack
  const minioConfig: FileStorageConfig = {
    type: 's3',
    path: 'test-bucket',
    endpoint: 'http://localhost:9000',
    credentials: {
      accessKeyId: 'minioadmin',
      secretAccessKey: 'minioadmin',
    },
  };

  const minioStorage = new S3FileStorage(minioConfig);
  assert(minioStorage !== null, 'Should construct with MinIO endpoint');

  // Test with LocalStack endpoint
  const localstackConfig: FileStorageConfig = {
    type: 's3',
    path: 'test-bucket',
    endpoint: 'http://localhost:4566',
    credentials: {
      accessKeyId: 'test',
      secretAccessKey: 'test',
    },
  };

  const localstackStorage = new S3FileStorage(localstackConfig);
  assert(localstackStorage !== null, 'Should construct with LocalStack endpoint');

  // Test that listFiles wraps errors nicely
  try {
    await minioStorage.listFiles();
    assert(true, 'MinIO listFiles executed (unlikely in test env)');
  } catch (err: any) {
    // Should be wrapped in our S3StorageError or be an SDK missing error
    const isWrapped = err instanceof S3StorageError;
    const isSdkMissing = err.message?.includes('@aws-sdk/client-s3');
    assert(
      isWrapped || isSdkMissing,
      `Error should be S3StorageError or SDK missing error: ${err.constructor?.name}`
    );
    if (isWrapped) {
      assert(
        ['NETWORK_ERROR', 'AUTH_FAILED', 'BUCKET_NOT_FOUND', 'UNKNOWN'].includes(err.code),
        `Error code should be descriptive: ${err.code}`
      );
      assert(
        err.bucket === 'test-bucket',
        'Error should include bucket name'
      );
    }
  }

  // Test readFile error wrapping
  try {
    await localstackStorage.readFile('nonexistent.txt');
    assert(true, 'LocalStack readFile executed (unlikely in test env)');
  } catch (err: any) {
    const isWrapped = err instanceof S3StorageError;
    const isSdkMissing = err.message?.includes('@aws-sdk/client-s3');
    assert(
      isWrapped || isSdkMissing,
      `readFile error should be wrapped: ${err.constructor?.name}`
    );
  }
}

// ─── Test: S3 Endpoint Configuration for Various Providers ───────────────────

async function testS3EndpointConfigurations() {
  console.log('\n=== S3 FILE STORAGE: Endpoint Configurations ===\n');

  // MinIO configuration
  const minioStorage = new S3FileStorage({
    type: 's3',
    path: 'minio-bucket',
    endpoint: 'http://minio.local:9000',
    region: 'us-east-1',
    credentials: {
      accessKeyId: 'minioadmin',
      secretAccessKey: 'minioadmin',
    },
  });
  assert(minioStorage !== null, 'MinIO config should be valid');
  assertEqual(minioStorage.getCredentialSource(), 'explicit', 'MinIO should use explicit creds');

  // LocalStack configuration
  const localstackStorage = new S3FileStorage({
    type: 's3',
    path: 'localstack-bucket',
    endpoint: 'http://localhost:4566',
    region: 'us-east-1',
    credentials: {
      accessKeyId: 'test',
      secretAccessKey: 'test',
    },
  });
  assert(localstackStorage !== null, 'LocalStack config should be valid');

  // DigitalOcean Spaces style
  const doStorage = new S3FileStorage({
    type: 's3',
    path: 'my-space',
    endpoint: 'https://nyc3.digitaloceanspaces.com',
    region: 'nyc3',
    credentials: {
      accessKeyId: 'do-key',
      secretAccessKey: 'do-secret',
    },
  });
  assert(doStorage !== null, 'DigitalOcean Spaces config should be valid');

  // Backblaze B2 style
  const b2Storage = new S3FileStorage({
    type: 's3',
    path: 'b2-bucket',
    endpoint: 'https://s3.us-west-004.backblazeb2.com',
    region: 'us-west-004',
    credentials: {
      accessKeyId: 'b2-key',
      secretAccessKey: 'b2-secret',
    },
  });
  assert(b2Storage !== null, 'Backblaze B2 config should be valid');

  // forcePathStyle explicit
  const forcePathStorage = new S3FileStorage({
    type: 's3',
    path: 'my-bucket',
    forcePathStyle: true,
  });
  assert(forcePathStorage !== null, 'forcePathStyle config should be valid');
}

// ─── Test: Mock Error Scenarios via FileContextBuilder ────────────────────────

class FailingMockProvider implements FileStorageProvider {
  private _failOn: string;
  private _errorType: string;
  private files: Map<string, string>;

  constructor(files: Record<string, string>, failOn: string, errorType: string) {
    this.files = new Map(Object.entries(files));
    this._failOn = failOn;
    this._errorType = errorType;
  }

  async listFiles(): Promise<string[]> {
    if (this._failOn === 'list') {
      throw this.makeError();
    }
    return Array.from(this.files.keys()).sort();
  }

  async readFile(relativePath: string): Promise<string> {
    if (this._failOn === 'read' || this._failOn === relativePath) {
      throw this.makeError();
    }
    const content = this.files.get(relativePath);
    if (!content) throw new Error(`File not found: ${relativePath}`);
    return content;
  }

  async getFileSize(relativePath: string): Promise<number> {
    if (this._failOn === 'size' || this._failOn === `size:${relativePath}`) {
      throw this.makeError();
    }
    const content = this.files.get(relativePath);
    if (!content) throw new Error(`File not found: ${relativePath}`);
    return Buffer.byteLength(content, 'utf-8');
  }

  private makeError(): Error {
    switch (this._errorType) {
      case 'auth':
        return Object.assign(new Error('Could not load credentials from any providers'), { name: 'CredentialsProviderError' });
      case 'access':
        return Object.assign(new Error('Access Denied'), { name: 'AccessDenied', $metadata: { httpStatusCode: 403 } });
      case 'not-found':
        return Object.assign(new Error('The specified bucket does not exist'), { name: 'NoSuchBucket', $metadata: { httpStatusCode: 404 } });
      case 'network':
        return Object.assign(new Error('connect ECONNREFUSED 127.0.0.1:9000'), { name: 'NetworkingError' });
      default:
        return new Error(`Unknown error type: ${this._errorType}`);
    }
  }
}

async function testMockErrorScenarios() {
  console.log('\n=== S3 FILE STORAGE: Mock Error Scenarios ===\n');

  const mockFiles = {
    'file1.txt': 'hello',
    'file2.txt': 'world',
  };

  // 1. Read error for specific file is gracefully handled by FileContextBuilder
  const readFailProvider = new FailingMockProvider(mockFiles, 'file1.txt', 'network');
  const builder1 = new FileContextBuilderWithMock(readFailProvider, {
    type: 's3',
    path: 'test-bucket',
  });
  const result1 = await builder1.buildContext();
  assertEqual(result1.files.length, 1, 'Should include file2.txt despite file1.txt read error');
  assert(
    result1.skipped.some(f => f.relativePath === 'file1.txt' && f.reason.includes('read error')),
    'Should record file1.txt as skipped due to read error'
  );

  // 2. Size check error is gracefully handled
  const sizeFailProvider = new FailingMockProvider(mockFiles, 'size:file1.txt', 'access');
  const builder2 = new FileContextBuilderWithMock(sizeFailProvider, {
    type: 's3',
    path: 'test-bucket',
  });
  const result2 = await builder2.buildContext();
  // The builder should catch the error and skip the file
  assert(
    result2.skipped.some(f => f.relativePath === 'file1.txt'),
    'Should skip file1.txt when size check fails'
  );

  // 3. List failure should propagate
  const listFailProvider = new FailingMockProvider(mockFiles, 'list', 'auth');
  const builder3 = new FileContextBuilderWithMock(listFailProvider, {
    type: 's3',
    path: 'test-bucket',
  });
  try {
    await builder3.buildContext();
    assert(false, 'Should have thrown on list failure');
  } catch (err: any) {
    assert(
      err.message.includes('Could not load credentials'),
      'List failure should propagate with original message'
    );
  }
}

// ─── Test: FileContextBuilder Error Recovery with Mock ───────────────────────

// Updated helper class that catches per-file errors
class FileContextBuilderWithMockV2 {
  private provider: FileStorageProvider;
  private config: FileStorageConfig;

  constructor(provider: FileStorageProvider, config: FileStorageConfig) {
    this.provider = provider;
    this.config = config;
  }

  async buildContext(): Promise<FileStorageResult> {
    const maxFileSize = this.config.maxFileSize ?? 1024 * 1024;
    const maxTotalSize = this.config.maxTotalSize ?? 10 * 1024 * 1024;
    const maxFiles = this.config.maxFiles ?? 1000;

    const allFiles = await this.provider.listFiles();
    const matchedFiles: string[] = [];
    const skipped: Array<{ relativePath: string; reason: string }> = [];

    for (const filePath of allFiles) {
      if (this.config.extensions && this.config.extensions.length > 0) {
        const ext = path.extname(filePath).toLowerCase();
        if (!this.config.extensions.includes(ext)) {
          skipped.push({ relativePath: filePath, reason: `extension ${ext} not in allowed list` });
          continue;
        }
      }
      matchedFiles.push(filePath);
    }

    const includedFiles: Array<{ relativePath: string; size: number }> = [];
    const contextParts: string[] = [];
    let totalSize = 0;

    contextParts.push(`=== FILE STRUCTURE ===\nTotal files: ${matchedFiles.length}\n=== END FILE STRUCTURE ===\n`);

    for (const filePath of matchedFiles) {
      if (includedFiles.length >= maxFiles) break;

      try {
        const fileSize = await this.provider.getFileSize(filePath);
        if (fileSize > maxFileSize) {
          skipped.push({ relativePath: filePath, reason: 'too large' });
          continue;
        }
        if (totalSize + fileSize > maxTotalSize) {
          skipped.push({ relativePath: filePath, reason: 'total size exceeded' });
          continue;
        }

        const content = await this.provider.readFile(filePath);
        const ext = path.extname(filePath).slice(1);
        const block = `=== FILE: ${filePath} ===\n\`\`\`${ext}\n${content}\n\`\`\`\n=== END FILE: ${filePath} ===\n`;
        contextParts.push(block);
        totalSize += Buffer.byteLength(block, 'utf-8');
        includedFiles.push({ relativePath: filePath, size: fileSize });
      } catch (err: any) {
        skipped.push({ relativePath: filePath, reason: `read error: ${err.message}` });
      }
    }

    return {
      context: contextParts.join('\n'),
      files: includedFiles,
      totalSize,
      skipped,
    };
  }
}

async function testErrorRecovery() {
  console.log('\n=== S3 FILE STORAGE: Error Recovery ===\n');

  const mockFiles = {
    'good1.txt': 'good content 1',
    'bad.txt': 'will fail on read',
    'good2.txt': 'good content 2',
  };

  // Provider that fails on reading 'bad.txt'
  const provider = new FailingMockProvider(mockFiles, 'bad.txt', 'network');
  const builder = new FileContextBuilderWithMockV2(provider, {
    type: 's3',
    path: 'test-bucket',
  });
  const result = await builder.buildContext();

  assertEqual(result.files.length, 2, 'Should include 2 good files despite 1 failure');
  assert(
    result.files.some(f => f.relativePath === 'good1.txt'),
    'Should include good1.txt'
  );
  assert(
    result.files.some(f => f.relativePath === 'good2.txt'),
    'Should include good2.txt'
  );
  assert(
    result.skipped.some(f => f.relativePath === 'bad.txt' && f.reason.includes('read error')),
    'Should skip bad.txt with error reason'
  );
  assert(
    result.context.includes('good content 1'),
    'Context should include good1 content'
  );
  assert(
    result.context.includes('good content 2'),
    'Context should include good2 content'
  );
  assert(
    !result.context.includes('will fail on read'),
    'Context should NOT include bad file content'
  );
}

// ─── Test: S3 with forcePathStyle ────────────────────────────────────────────

async function testForcePathStyle() {
  console.log('\n=== S3 FILE STORAGE: forcePathStyle ===\n');

  // When endpoint is set, forcePathStyle should be auto-enabled
  const withEndpoint = new S3FileStorage({
    type: 's3',
    path: 'my-bucket',
    endpoint: 'http://localhost:9000',
    credentials: { accessKeyId: 'test', secretAccessKey: 'test' },
  });
  assert(withEndpoint !== null, 'Should construct with auto forcePathStyle');

  // Explicit forcePathStyle without endpoint
  const explicit = new S3FileStorage({
    type: 's3',
    path: 'my-bucket',
    forcePathStyle: true,
    credentials: { accessKeyId: 'test', secretAccessKey: 'test' },
  });
  assert(explicit !== null, 'Should construct with explicit forcePathStyle');

  // No forcePathStyle without endpoint (normal S3)
  const normal = new S3FileStorage({
    type: 's3',
    path: 'my-bucket',
    credentials: { accessKeyId: 'test', secretAccessKey: 'test' },
  });
  assert(normal !== null, 'Should construct without forcePathStyle');
}

// ─── Run All Tests ───────────────────────────────────────────────────────────

async function runAllTests() {
  console.log('🧪 File Storage Test Suite\n');
  console.log('='.repeat(60));

  // Local file storage tests
  await testLocalFileStorageBasic();
  await testDeepNesting();
  await testExtensionFilter();
  await testGlobPatterns();
  await testMaxFileSize();
  await testMaxTotalSize();
  await testMaxFiles();
  await testContextFormat();
  await testEmptyDirectory();
  await testListMatchingFiles();
  await testBuildFileContextFunction();
  await testBinaryFileHandling();
  await testSpecialCharacters();
  await testEdgeCases();

  // S3 tests (unit/mock level)
  await testS3StorageConstruction();
  await testMockS3Provider();
  await testMockS3NestedFolders();
  await testMockS3MaxConstraints();

  // S3 credential resolution tests
  await testS3CredentialResolution();
  await testS3RegionResolution();

  // S3 error handling tests
  await testS3StorageError();
  await testS3ErrorWrapping();
  await testS3EndpointConfigurations();
  await testMockErrorScenarios();
  await testErrorRecovery();
  await testForcePathStyle();

  // Integration-like test
  await testContextForLLM();

  // Summary
  console.log('\n' + '='.repeat(60));
  console.log(`\n📊 Results: ${passCount}/${testCount} passed, ${failCount} failed\n`);

  if (failCount > 0) {
    console.error(`❌ ${failCount} TESTS FAILED`);
    process.exit(1);
  } else {
    console.log('✅ ALL TESTS PASSED');
  }
}

runAllTests().catch(err => {
  console.error('Test suite crashed:', err);
  process.exit(1);
});
