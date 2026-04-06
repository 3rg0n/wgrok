"""Domain/email pattern matching for wgrok allowlist."""

import logging

logger = logging.getLogger(__name__)


class Allowlist:
    """Validates email addresses against a list of allowed patterns.

    Supported pattern formats:
    - "domain.tld" -> matches *@domain.tld (bare domain)
    - "*@domain.tld" -> wildcard prefix match
    - "user@domain.tld" -> exact match (case-insensitive)

    Dangerous patterns with [, ], or ? are rejected.
    """

    def __init__(self, patterns: list[str]) -> None:
        self._patterns: list[tuple[str, str]] = []  # (type, pattern) pairs
        for p in patterns:
            p = p.strip()
            if not p:
                continue
            # Reject patterns with dangerous characters
            if "[" in p or "]" in p or "?" in p:
                logger.warning(f"Rejecting dangerous allowlist pattern: {p}")
                continue
            # Normalize and store with type
            if p.startswith("*@"):
                # Wildcard prefix: *@domain.tld
                domain = p[2:]  # Extract domain part
                self._patterns.append(("wildcard_prefix", domain.lower()))
            elif "@" in p:
                # Exact match: user@domain.tld
                self._patterns.append(("exact", p.lower()))
            else:
                # Bare domain: domain.tld (equivalent to *@domain.tld)
                self._patterns.append(("bare_domain", p.lower()))

    def is_allowed(self, email: str) -> bool:
        """Check if an email address matches any allowed pattern (case-insensitive)."""
        email_lower = email.lower()
        for pattern_type, pattern_value in self._patterns:
            if pattern_type == "exact" and email_lower == pattern_value:
                return True
            if pattern_type in ("wildcard_prefix", "bare_domain") and email_lower.endswith(f"@{pattern_value}"):
                return True
        return False
