/**
 * Acme Analytics - Event Processing Service
 *
 * This service handles incoming events from the ingestion API,
 * validates them against customer schemas, and publishes to Kafka.
 */

import { EventEmitter } from 'events';

interface AnalyticsEvent {
  event_type: string;
  timestamp: string;
  user_id: string;
  properties: Record<string, unknown>;
  metadata?: {
    sdk_version?: string;
    ip_address?: string;
    user_agent?: string;
  };
}

interface ValidationResult {
  valid: boolean;
  errors: string[];
}

interface ProcessingStats {
  total_received: number;
  total_accepted: number;
  total_rejected: number;
  avg_latency_ms: number;
  events_per_second: number;
}

class EventProcessor extends EventEmitter {
  private stats: ProcessingStats = {
    total_received: 0,
    total_accepted: 0,
    total_rejected: 0,
    avg_latency_ms: 0,
    events_per_second: 0,
  };

  private schemas: Map<string, Record<string, unknown>> = new Map();
  private rateLimiter: TokenBucketRateLimiter;

  constructor(
    private readonly kafkaProducer: KafkaProducer,
    private readonly schemaRegistry: SchemaRegistry,
    config: { maxEventsPerSecond: number }
  ) {
    super();
    this.rateLimiter = new TokenBucketRateLimiter(config.maxEventsPerSecond);
  }

  async processBatch(events: AnalyticsEvent[], apiKey: string): Promise<{
    accepted: number;
    rejected: number;
    errors: Array<{ index: number; error: string }>;
  }> {
    const startTime = Date.now();
    const errors: Array<{ index: number; error: string }> = [];
    let accepted = 0;
    let rejected = 0;

    // Rate limit check
    if (!this.rateLimiter.tryConsume(events.length)) {
      throw new RateLimitError(
        `Rate limit exceeded: ${events.length} events requested, ` +
        `${this.rateLimiter.available()} available`
      );
    }

    // Process each event
    for (let i = 0; i < events.length; i++) {
      const event = events[i];

      // Validate against schema
      const validation = await this.validateEvent(event, apiKey);
      if (!validation.valid) {
        errors.push({ index: i, error: validation.errors.join('; ') });
        rejected++;
        continue;
      }

      // Enrich with metadata
      const enrichedEvent = this.enrichEvent(event);

      // Publish to Kafka
      try {
        await this.kafkaProducer.send({
          topic: `events-${apiKey}`,
          key: event.user_id,
          value: JSON.stringify(enrichedEvent),
          timestamp: event.timestamp,
        });
        accepted++;
      } catch (err) {
        errors.push({ index: i, error: 'Failed to publish event' });
        rejected++;
        this.emit('publish_error', { event, error: err });
      }
    }

    // Update stats
    const latency = Date.now() - startTime;
    this.updateStats(events.length, accepted, rejected, latency);

    return { accepted, rejected, errors };
  }

  private async validateEvent(
    event: AnalyticsEvent,
    apiKey: string
  ): Promise<ValidationResult> {
    const errors: string[] = [];

    // Required fields
    if (!event.event_type) errors.push('Missing required field: event_type');
    if (!event.timestamp) errors.push('Missing required field: timestamp');
    if (!event.user_id) errors.push('Missing required field: user_id');

    // Timestamp format
    if (event.timestamp && isNaN(Date.parse(event.timestamp))) {
      errors.push('Invalid timestamp format: must be ISO 8601');
    }

    // Event type naming convention
    if (event.event_type && !/^[a-z][a-z0-9_]*$/.test(event.event_type)) {
      errors.push('Invalid event_type: must be snake_case');
    }

    // Custom schema validation
    const schema = await this.getSchema(apiKey, event.event_type);
    if (schema) {
      const schemaErrors = this.validateAgainstSchema(event.properties, schema);
      errors.push(...schemaErrors);
    }

    return { valid: errors.length === 0, errors };
  }

  private enrichEvent(event: AnalyticsEvent): AnalyticsEvent & {
    _enriched: {
      received_at: string;
      geo?: { country: string; region: string; city: string };
      device?: { type: string; os: string; browser: string };
    };
  } {
    return {
      ...event,
      _enriched: {
        received_at: new Date().toISOString(),
        // Geo and device enrichment would use actual lookup services
      },
    };
  }

  private async getSchema(
    apiKey: string,
    eventType: string
  ): Promise<Record<string, unknown> | null> {
    const cacheKey = `${apiKey}:${eventType}`;
    if (this.schemas.has(cacheKey)) {
      return this.schemas.get(cacheKey)!;
    }

    const schema = await this.schemaRegistry.getSchema(apiKey, eventType);
    if (schema) {
      this.schemas.set(cacheKey, schema);
    }
    return schema;
  }

  private validateAgainstSchema(
    properties: Record<string, unknown>,
    schema: Record<string, unknown>
  ): string[] {
    // Simplified schema validation - real implementation uses JSON Schema
    const errors: string[] = [];
    const required = (schema.required as string[]) || [];

    for (const field of required) {
      if (!(field in properties)) {
        errors.push(`Missing required property: ${field}`);
      }
    }

    return errors;
  }

  private updateStats(
    received: number,
    accepted: number,
    rejected: number,
    latencyMs: number
  ): void {
    this.stats.total_received += received;
    this.stats.total_accepted += accepted;
    this.stats.total_rejected += rejected;
    this.stats.avg_latency_ms =
      (this.stats.avg_latency_ms * 0.9) + (latencyMs * 0.1); // EMA
  }

  getStats(): ProcessingStats {
    return { ...this.stats };
  }
}

class TokenBucketRateLimiter {
  private tokens: number;
  private lastRefill: number;

  constructor(private readonly maxTokens: number) {
    this.tokens = maxTokens;
    this.lastRefill = Date.now();
  }

  tryConsume(count: number): boolean {
    this.refill();
    if (this.tokens >= count) {
      this.tokens -= count;
      return true;
    }
    return false;
  }

  available(): number {
    this.refill();
    return Math.floor(this.tokens);
  }

  private refill(): void {
    const now = Date.now();
    const elapsed = (now - this.lastRefill) / 1000;
    this.tokens = Math.min(this.maxTokens, this.tokens + elapsed * this.maxTokens);
    this.lastRefill = now;
  }
}

class RateLimitError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'RateLimitError';
  }
}

// Placeholder interfaces for dependencies
interface KafkaProducer {
  send(message: {
    topic: string;
    key: string;
    value: string;
    timestamp: string;
  }): Promise<void>;
}

interface SchemaRegistry {
  getSchema(
    apiKey: string,
    eventType: string
  ): Promise<Record<string, unknown> | null>;
}

export { EventProcessor, AnalyticsEvent, ProcessingStats, RateLimitError };
