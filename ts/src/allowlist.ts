interface NormalizedPattern {
  type: 'exact' | 'wildcard_prefix' | 'bare_domain';
  value: string;
}

export class Allowlist {
  private patterns: NormalizedPattern[];

  constructor(patterns: string[]) {
    this.patterns = [];
    for (const p of patterns) {
      const trimmed = p.trim();
      if (!trimmed) {
        continue;
      }
      // Reject patterns with dangerous characters
      if (/[\[\]?]/.test(trimmed)) {
        console.warn(`Rejecting dangerous allowlist pattern: ${trimmed}`);
        continue;
      }
      const lower = trimmed.toLowerCase();
      if (lower.startsWith('*@')) {
        // Wildcard prefix: *@domain.tld
        this.patterns.push({
          type: 'wildcard_prefix',
          value: lower.slice(2),
        });
      } else if (lower.includes('@')) {
        // Exact match: user@domain.tld
        this.patterns.push({
          type: 'exact',
          value: lower,
        });
      } else {
        // Bare domain: domain.tld
        this.patterns.push({
          type: 'bare_domain',
          value: lower,
        });
      }
    }
  }

  isAllowed(email: string): boolean {
    const emailLower = email.toLowerCase();
    return this.patterns.some((pattern) => this.matchesPattern(pattern, emailLower));
  }

  private matchesPattern(pattern: NormalizedPattern, email: string): boolean {
    switch (pattern.type) {
      case 'exact':
        return email === pattern.value;
      case 'wildcard_prefix':
        return email.endsWith(`@${pattern.value}`);
      case 'bare_domain':
        return email.endsWith(`@${pattern.value}`);
    }
  }
}
