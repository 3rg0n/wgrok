import type { Logger } from 'webex-message-handler';

export class NdjsonLogger implements Logger {
  constructor(private module: string = 'wgrok') {}

  debug(message: string, fields?: Record<string, string>): void {
    this.write('DEBUG', message, fields);
  }
  info(message: string, fields?: Record<string, string>): void {
    this.write('INFO', message, fields);
  }
  warn(message: string, fields?: Record<string, string>): void {
    this.write('WARNING', message, fields);
  }
  error(message: string, fields?: Record<string, string>): void {
    this.write('ERROR', message, fields);
  }

  private write(level: string, msg: string, fields?: Record<string, string>): void {
    const line = JSON.stringify({
      ts: new Date().toISOString(),
      level,
      msg,
      module: this.module,
      ...fields,
    });
    process.stderr.write(line + '\n');
  }
}

export class MinLevelLogger implements Logger {
  private ndjson: NdjsonLogger;

  constructor(module?: string) {
    this.ndjson = new NdjsonLogger(module);
  }

  debug(): void {}
  info(): void {}
  warn(message: string, fields?: Record<string, string>): void {
    this.ndjson.warn(message, fields);
  }
  error(message: string, fields?: Record<string, string>): void {
    this.ndjson.error(message, fields);
  }
}

/** Fully silent logger for use in tests only. */
export const noopLogger: Logger = {
  debug() {},
  info() {},
  warn() {},
  error() {},
};

export function getLogger(debug: boolean, module?: string): Logger {
  return debug ? new NdjsonLogger(module) : new MinLevelLogger(module);
}
