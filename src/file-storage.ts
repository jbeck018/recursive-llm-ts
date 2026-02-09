import * as fs from 'fs';
import * as path from 'path';

// Dynamic loader for optional @aws-sdk/client-s3 dependency.
// Uses require() to avoid TypeScript resolving the import at compile time.
function loadS3SDK(): any {
  try {
    // eslint-disable-next-line @typescript-eslint/no-var-requires
    return require('@aws-sdk/client-s3');
  } catch {
    throw new Error(
      'S3 file storage requires @aws-sdk/client-s3. Install it with: npm install @aws-sdk/client-s3'
    );
  }
}

// ─── Types ───────────────────────────────────────────────────────────────────

export interface FileEntry {
  /** Relative path from the root of the storage source */
  relativePath: string;
  /** File content as a string */
  content: string;
  /** Size in bytes */
  size: number;
}

export interface FileStorageConfig {
  /** Storage type: 'local' or 's3' */
  type: 'local' | 's3';

  /** For local: root directory path. For S3: bucket name */
  path: string;

  /** For S3: the prefix (folder path) within the bucket */
  prefix?: string;

  /** For S3: AWS region */
  region?: string;

  /** For S3: explicit credentials (falls back to env/default chain) */
  credentials?: {
    accessKeyId: string;
    secretAccessKey: string;
    sessionToken?: string;
  };

  /** For S3: custom endpoint URL (e.g. for MinIO, LocalStack) */
  endpoint?: string;

  /** Glob patterns to include (e.g. ['*.ts', '*.md']). Default: all files */
  includePatterns?: string[];

  /** Glob patterns to exclude (e.g. ['node_modules/**', '*.log']) */
  excludePatterns?: string[];

  /** Maximum file size in bytes to include (default: 1MB) */
  maxFileSize?: number;

  /** Maximum total context size in bytes (default: 10MB) */
  maxTotalSize?: number;

  /** Maximum number of files to include (default: 1000) */
  maxFiles?: number;

  /** File extensions to include (e.g. ['.ts', '.md', '.txt']). Overrides includePatterns for extension matching */
  extensions?: string[];
}

export interface FileStorageResult {
  /** The built context string containing all file contents */
  context: string;
  /** List of files that were included */
  files: Array<{ relativePath: string; size: number }>;
  /** Total size of all file contents */
  totalSize: number;
  /** Files that were skipped and why */
  skipped: Array<{ relativePath: string; reason: string }>;
}

// ─── Glob Matching ───────────────────────────────────────────────────────────

/**
 * Simple glob pattern matcher supporting *, **, and ? wildcards.
 * Does not depend on external libraries.
 */
function globToRegex(pattern: string): RegExp {
  let regex = '';
  let i = 0;
  while (i < pattern.length) {
    const c = pattern[i];
    if (c === '*') {
      if (pattern[i + 1] === '*') {
        // ** matches any number of path segments
        if (pattern[i + 2] === '/') {
          regex += '(?:.+/)?';
          i += 3;
        } else {
          regex += '.*';
          i += 2;
        }
      } else {
        // * matches anything except /
        regex += '[^/]*';
        i += 1;
      }
    } else if (c === '?') {
      regex += '[^/]';
      i += 1;
    } else if (c === '.') {
      regex += '\\.';
      i += 1;
    } else {
      regex += c;
      i += 1;
    }
  }
  return new RegExp('^' + regex + '$');
}

function matchesGlob(filePath: string, pattern: string): boolean {
  const regex = globToRegex(pattern);
  // Match against full relative path
  if (regex.test(filePath)) return true;
  // If pattern has no path separators, also match against just the filename
  if (!pattern.includes('/')) {
    return regex.test(path.basename(filePath));
  }
  return false;
}

function matchesAnyGlob(filePath: string, patterns: string[]): boolean {
  return patterns.some(p => matchesGlob(filePath, p));
}

// ─── File Storage Provider Interface ─────────────────────────────────────────

export interface FileStorageProvider {
  listFiles(): Promise<string[]>;
  readFile(relativePath: string): Promise<string>;
  getFileSize(relativePath: string): Promise<number>;
}

// ─── Local File Storage ──────────────────────────────────────────────────────

export class LocalFileStorage implements FileStorageProvider {
  private rootPath: string;

  constructor(rootPath: string) {
    this.rootPath = path.resolve(rootPath);
  }

  async listFiles(): Promise<string[]> {
    const files: string[] = [];
    await this.walkDir(this.rootPath, files);
    return files.sort();
  }

  private async walkDir(dir: string, files: string[]): Promise<void> {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        await this.walkDir(fullPath, files);
      } else if (entry.isFile()) {
        const relativePath = path.relative(this.rootPath, fullPath);
        files.push(relativePath);
      }
    }
  }

  async readFile(relativePath: string): Promise<string> {
    const fullPath = path.join(this.rootPath, relativePath);
    return fs.readFileSync(fullPath, 'utf-8');
  }

  async getFileSize(relativePath: string): Promise<number> {
    const fullPath = path.join(this.rootPath, relativePath);
    const stat = fs.statSync(fullPath);
    return stat.size;
  }
}

// ─── S3 File Storage ─────────────────────────────────────────────────────────

/**
 * S3FileStorage uses the AWS SDK v3 via dynamic import.
 * If @aws-sdk/client-s3 is not installed, it throws a clear error.
 */
export class S3FileStorage implements FileStorageProvider {
  private bucket: string;
  private prefix: string;
  private region: string;
  private credentials?: {
    accessKeyId: string;
    secretAccessKey: string;
    sessionToken?: string;
  };
  private endpoint?: string;
  private s3Client: any = null;

  constructor(config: FileStorageConfig) {
    this.bucket = config.path;
    this.prefix = config.prefix || '';
    this.region = config.region || process.env.AWS_REGION || 'us-east-1';
    this.credentials = config.credentials;
    this.endpoint = config.endpoint;
  }

  private async getClient(): Promise<any> {
    if (this.s3Client) return this.s3Client;

    const s3Module = loadS3SDK();
    const clientConfig: any = { region: this.region };

    if (this.credentials) {
      clientConfig.credentials = this.credentials;
    }
    if (this.endpoint) {
      clientConfig.endpoint = this.endpoint;
      clientConfig.forcePathStyle = true;
    }

    this.s3Client = new s3Module.S3Client(clientConfig);
    return this.s3Client;
  }

  async listFiles(): Promise<string[]> {
    const client = await this.getClient();
    const s3Module = loadS3SDK();

    const files: string[] = [];
    let continuationToken: string | undefined;

    do {
      const command = new s3Module.ListObjectsV2Command({
        Bucket: this.bucket,
        Prefix: this.prefix,
        ContinuationToken: continuationToken,
      });

      const response = await client.send(command);

      if (response.Contents) {
        for (const obj of response.Contents) {
          if (obj.Key && !obj.Key.endsWith('/')) {
            // Get relative path by removing the prefix
            const relativePath = this.prefix
              ? obj.Key.substring(this.prefix.length).replace(/^\//, '')
              : obj.Key;
            if (relativePath) {
              files.push(relativePath);
            }
          }
        }
      }

      continuationToken = response.IsTruncated
        ? response.NextContinuationToken
        : undefined;
    } while (continuationToken);

    return files.sort();
  }

  async readFile(relativePath: string): Promise<string> {
    const client = await this.getClient();
    const s3Module = loadS3SDK();

    const key = this.prefix
      ? `${this.prefix.replace(/\/$/, '')}/${relativePath}`
      : relativePath;

    const command = new s3Module.GetObjectCommand({
      Bucket: this.bucket,
      Key: key,
    });

    const response = await client.send(command);
    const body = response.Body;

    if (!body) {
      throw new Error(`Empty response body for s3://${this.bucket}/${key}`);
    }

    // Handle different stream types
    if (typeof body.transformToString === 'function') {
      return await body.transformToString('utf-8');
    }

    // Fallback: collect stream chunks
    const chunks: Buffer[] = [];
    for await (const chunk of body as AsyncIterable<Buffer>) {
      chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
    }
    return Buffer.concat(chunks).toString('utf-8');
  }

  async getFileSize(relativePath: string): Promise<number> {
    const client = await this.getClient();
    const s3Module = loadS3SDK();

    const key = this.prefix
      ? `${this.prefix.replace(/\/$/, '')}/${relativePath}`
      : relativePath;

    const command = new s3Module.HeadObjectCommand({
      Bucket: this.bucket,
      Key: key,
    });

    const response = await client.send(command);
    return response.ContentLength || 0;
  }
}

// ─── File Context Builder ────────────────────────────────────────────────────

const DEFAULT_MAX_FILE_SIZE = 1 * 1024 * 1024; // 1MB
const DEFAULT_MAX_TOTAL_SIZE = 10 * 1024 * 1024; // 10MB
const DEFAULT_MAX_FILES = 1000;

/**
 * FileContextBuilder reads files from a storage provider, applies filters,
 * and builds a structured context string for LLM consumption.
 */
export class FileContextBuilder {
  private provider: FileStorageProvider;
  private config: FileStorageConfig;

  constructor(config: FileStorageConfig) {
    this.config = config;
    this.provider = this.createProvider(config);
  }

  private createProvider(config: FileStorageConfig): FileStorageProvider {
    switch (config.type) {
      case 'local':
        return new LocalFileStorage(config.path);
      case 's3':
        return new S3FileStorage(config);
      default:
        throw new Error(`Unknown file storage type: ${(config as any).type}`);
    }
  }

  /**
   * Build a structured context string from all matching files.
   * Files are formatted with clear delimiters so the LLM can reference them.
   */
  async buildContext(): Promise<FileStorageResult> {
    const maxFileSize = this.config.maxFileSize ?? DEFAULT_MAX_FILE_SIZE;
    const maxTotalSize = this.config.maxTotalSize ?? DEFAULT_MAX_TOTAL_SIZE;
    const maxFiles = this.config.maxFiles ?? DEFAULT_MAX_FILES;

    // 1. List all files
    const allFiles = await this.provider.listFiles();

    // 2. Apply filters
    const matchedFiles: string[] = [];
    const skipped: Array<{ relativePath: string; reason: string }> = [];

    for (const filePath of allFiles) {
      // Extension filter
      if (this.config.extensions && this.config.extensions.length > 0) {
        const ext = path.extname(filePath).toLowerCase();
        if (!this.config.extensions.includes(ext)) {
          skipped.push({ relativePath: filePath, reason: `extension ${ext} not in allowed list` });
          continue;
        }
      }

      // Include patterns
      if (this.config.includePatterns && this.config.includePatterns.length > 0) {
        if (!matchesAnyGlob(filePath, this.config.includePatterns)) {
          skipped.push({ relativePath: filePath, reason: 'did not match include patterns' });
          continue;
        }
      }

      // Exclude patterns
      if (this.config.excludePatterns && this.config.excludePatterns.length > 0) {
        if (matchesAnyGlob(filePath, this.config.excludePatterns)) {
          skipped.push({ relativePath: filePath, reason: 'matched exclude pattern' });
          continue;
        }
      }

      matchedFiles.push(filePath);
    }

    // 3. Read files and build context
    const includedFiles: Array<{ relativePath: string; size: number }> = [];
    const contextParts: string[] = [];
    let totalSize = 0;
    let fileCount = 0;

    // Add file tree header
    const treeHeader = this.buildFileTree(matchedFiles);
    contextParts.push(treeHeader);
    totalSize += Buffer.byteLength(treeHeader, 'utf-8');

    for (const filePath of matchedFiles) {
      if (fileCount >= maxFiles) {
        skipped.push({ relativePath: filePath, reason: `max file count (${maxFiles}) reached` });
        continue;
      }

      try {
        const fileSize = await this.provider.getFileSize(filePath);

        if (fileSize > maxFileSize) {
          skipped.push({ relativePath: filePath, reason: `file size ${fileSize} exceeds max ${maxFileSize}` });
          continue;
        }

        if (totalSize + fileSize > maxTotalSize) {
          skipped.push({ relativePath: filePath, reason: `would exceed total size limit (${maxTotalSize})` });
          continue;
        }

        const content = await this.provider.readFile(filePath);
        const fileBlock = this.formatFileBlock(filePath, content);
        contextParts.push(fileBlock);

        totalSize += Buffer.byteLength(fileBlock, 'utf-8');
        fileCount++;
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

  /**
   * Build a file tree overview for the LLM to understand the folder structure.
   */
  private buildFileTree(files: string[]): string {
    const lines = [
      '=== FILE STRUCTURE ===',
      `Source: ${this.config.type === 's3' ? `s3://${this.config.path}/${this.config.prefix || ''}` : this.config.path}`,
      `Total files: ${files.length}`,
      '',
      'Files:',
    ];

    for (const file of files) {
      const depth = file.split('/').length - 1;
      const indent = '  '.repeat(depth);
      const basename = path.basename(file);
      lines.push(`${indent}${depth > 0 ? '├── ' : ''}${basename} (${file})`);
    }

    lines.push('', '=== END FILE STRUCTURE ===', '');
    return lines.join('\n');
  }

  /**
   * Format a single file's content with clear delimiters.
   */
  private formatFileBlock(relativePath: string, content: string): string {
    const ext = path.extname(relativePath).slice(1);
    return [
      `=== FILE: ${relativePath} ===`,
      `\`\`\`${ext}`,
      content,
      '```',
      `=== END FILE: ${relativePath} ===`,
      '',
    ].join('\n');
  }

  /**
   * Get just the list of files without reading content.
   * Useful for previewing what would be included.
   */
  async listMatchingFiles(): Promise<string[]> {
    const allFiles = await this.provider.listFiles();

    return allFiles.filter(filePath => {
      if (this.config.extensions && this.config.extensions.length > 0) {
        const ext = path.extname(filePath).toLowerCase();
        if (!this.config.extensions.includes(ext)) return false;
      }

      if (this.config.includePatterns && this.config.includePatterns.length > 0) {
        if (!matchesAnyGlob(filePath, this.config.includePatterns)) return false;
      }

      if (this.config.excludePatterns && this.config.excludePatterns.length > 0) {
        if (matchesAnyGlob(filePath, this.config.excludePatterns)) return false;
      }

      return true;
    });
  }
}

/**
 * Convenience function to build context from a file storage config.
 */
export async function buildFileContext(config: FileStorageConfig): Promise<FileStorageResult> {
  const builder = new FileContextBuilder(config);
  return builder.buildContext();
}
