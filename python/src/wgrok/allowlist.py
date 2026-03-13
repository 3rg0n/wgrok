"""Domain/email pattern matching for wgrok allowlist."""

from fnmatch import fnmatch


class Allowlist:
    """Validates email addresses against a list of allowed patterns.

    Supported pattern formats:
    - "domain.tld" -> matches *@domain.tld
    - "*@domain.tld" -> wildcard match
    - "user@domain.tld" -> exact match
    """

    def __init__(self, patterns: list[str]) -> None:
        self._patterns: list[str] = []
        for p in patterns:
            p = p.strip()
            if not p:
                continue
            if "@" not in p:
                self._patterns.append(f"*@{p}")
            else:
                self._patterns.append(p)

    def is_allowed(self, email: str) -> bool:
        """Check if an email address matches any allowed pattern (case-insensitive)."""
        email_lower = email.lower()
        return any(fnmatch(email_lower, pattern.lower()) for pattern in self._patterns)
