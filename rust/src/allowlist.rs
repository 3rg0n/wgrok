#[derive(Debug, Clone)]
enum PatternType {
    Exact(String),
    WildcardPrefix(String),
    BareDomain(String),
}

pub struct Allowlist {
    patterns: Vec<PatternType>,
}

impl Allowlist {
    pub fn new(patterns: &[String]) -> Self {
        let normalized: Vec<PatternType> = patterns
            .iter()
            .map(|p| p.trim().to_string())
            .filter(|p| !p.is_empty())
            .filter_map(|p| {
                let lower = p.to_lowercase();
                // Reject patterns with dangerous characters
                if lower.contains('[') || lower.contains(']') || lower.contains('?') {
                    eprintln!("Rejecting dangerous allowlist pattern: {}", p);
                    return None;
                }
                Some((p, lower))
            })
            .map(|(_, lower)| {
                if let Some(domain) = lower.strip_prefix("*@") {
                    // Wildcard prefix: *@domain.tld
                    PatternType::WildcardPrefix(domain.to_string())
                } else if lower.contains('@') {
                    // Exact match: user@domain.tld
                    PatternType::Exact(lower)
                } else {
                    // Bare domain: domain.tld
                    PatternType::BareDomain(lower)
                }
            })
            .collect();
        Self { patterns: normalized }
    }

    pub fn is_allowed(&self, email: &str) -> bool {
        let email_lower = email.to_lowercase();
        self.patterns
            .iter()
            .any(|pattern| self.matches_pattern(pattern, &email_lower))
    }

    fn matches_pattern(&self, pattern: &PatternType, email: &str) -> bool {
        match pattern {
            PatternType::Exact(p) => email == p,
            PatternType::WildcardPrefix(domain) => email.ends_with(&format!("@{}", domain)),
            PatternType::BareDomain(domain) => email.ends_with(&format!("@{}", domain)),
        }
    }
}
