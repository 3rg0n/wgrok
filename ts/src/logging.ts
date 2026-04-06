import type { Logger } from 'webex-message-handler';

export class NdjsonLogger implements Logger {
  constructor(private module: string = 'wgrok') {}

  debug(message: string): void {
    this.write('DEBUG', message);
  }
  info(message: string): void {
    this.write('INFO', message);
  }
  warn(message: string): void {
    this.write('WARNING', message);
  }
  error(message: string): void {
    this.write('ERROR', message);
  }

  private write(level: string, msg: string): void {
    const line = JSON.stringify({
      ts: new Date().toISOString(),
      level,
      msg,
      module: this.module,
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
  warn(message: string): void {
    this.ndjson.warn(message);
  }
  error(message: string): void {
    this.ndjson.error(message);
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
