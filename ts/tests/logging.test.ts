import { NdjsonLogger, noopLogger, getLogger } from '../src/logging';

describe('NdjsonLogger', () => {
  it('writes INFO JSON to stderr', () => {
    const chunks: string[] = [];
    const origWrite = process.stderr.write;
    process.stderr.write = ((data: string) => {
      chunks.push(data);
      return true;
    }) as typeof process.stderr.write;

    const logger = new NdjsonLogger('wgrok.test');
    logger.info('hello world');

    process.stderr.write = origWrite;
    const line = JSON.parse(chunks[0].trim());
    expect(line.level).toBe('INFO');
    expect(line.msg).toBe('hello world');
    expect(line.module).toBe('wgrok.test');
    expect(line.ts).toBeDefined();
  });

  it('writes DEBUG level', () => {
    const chunks: string[] = [];
    const origWrite = process.stderr.write;
    process.stderr.write = ((data: string) => {
      chunks.push(data);
      return true;
    }) as typeof process.stderr.write;

    new NdjsonLogger().debug('debug msg');

    process.stderr.write = origWrite;
    const line = JSON.parse(chunks[0].trim());
    expect(line.level).toBe('DEBUG');
  });

  it('writes WARNING level', () => {
    const chunks: string[] = [];
    const origWrite = process.stderr.write;
    process.stderr.write = ((data: string) => {
      chunks.push(data);
      return true;
    }) as typeof process.stderr.write;

    new NdjsonLogger().warn('warn msg');

    process.stderr.write = origWrite;
    const line = JSON.parse(chunks[0].trim());
    expect(line.level).toBe('WARNING');
  });

  it('writes ERROR level', () => {
    const chunks: string[] = [];
    const origWrite = process.stderr.write;
    process.stderr.write = ((data: string) => {
      chunks.push(data);
      return true;
    }) as typeof process.stderr.write;

    new NdjsonLogger().error('error msg');

    process.stderr.write = origWrite;
    const line = JSON.parse(chunks[0].trim());
    expect(line.level).toBe('ERROR');
  });

  it('uses default module "wgrok"', () => {
    const chunks: string[] = [];
    const origWrite = process.stderr.write;
    process.stderr.write = ((data: string) => {
      chunks.push(data);
      return true;
    }) as typeof process.stderr.write;

    new NdjsonLogger().info('test');

    process.stderr.write = origWrite;
    const line = JSON.parse(chunks[0].trim());
    expect(line.module).toBe('wgrok');
  });
});

describe('noopLogger', () => {
  it('produces no output', () => {
    const chunks: string[] = [];
    const origWrite = process.stderr.write;
    process.stderr.write = ((data: string) => {
      chunks.push(data);
      return true;
    }) as typeof process.stderr.write;

    noopLogger.debug('x');
    noopLogger.info('x');
    noopLogger.warn('x');
    noopLogger.error('x');

    process.stderr.write = origWrite;
    expect(chunks.length).toBe(0);
  });
});

describe('getLogger', () => {
  it('returns NdjsonLogger when debug=true', () => {
    const logger = getLogger(true);
    expect(logger).toBeInstanceOf(NdjsonLogger);
  });

  it('returns noopLogger when debug=false', () => {
    const logger = getLogger(false);
    expect(logger).toBe(noopLogger);
  });

  it('accepts custom module name', () => {
    const logger = getLogger(true, 'custom.mod');
    expect(logger).toBeInstanceOf(NdjsonLogger);
  });
});
