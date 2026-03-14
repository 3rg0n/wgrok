export class Allowlist {
  private patterns: string[];

  constructor(patterns: string[]) {
    this.patterns = patterns
      .map((p) => p.trim())
      .filter((p) => p.length > 0)
      .map((p) => (p.includes('@') ? p : `*@${p}`));
  }

  isAllowed(email: string): boolean {
    const emailLower = email.toLowerCase();
    return this.patterns.some((pattern) => matchPattern(pattern.toLowerCase(), emailLower));
  }
}

function matchPattern(pattern: string, value: string): boolean {
  const escaped = pattern.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*').replace(/\?/g, '.');
  return new RegExp(`^${escaped}$`).test(value);
}
