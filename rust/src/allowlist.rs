pub struct Allowlist {
    patterns: Vec<String>,
}

impl Allowlist {
    pub fn new(patterns: &[String]) -> Self {
        let normalized = patterns
            .iter()
            .map(|p| p.trim().to_string())
            .filter(|p| !p.is_empty())
            .map(|p| {
                if p.contains('@') {
                    p
                } else {
                    format!("*@{}", p)
                }
            })
            .collect();
        Self { patterns: normalized }
    }

    pub fn is_allowed(&self, email: &str) -> bool {
        let email_lower = email.to_lowercase();
        self.patterns
            .iter()
            .any(|pattern| glob_match(&pattern.to_lowercase(), &email_lower))
    }
}

fn glob_match(pattern: &str, value: &str) -> bool {
    let pi: Vec<char> = pattern.chars().collect();
    let vi: Vec<char> = value.chars().collect();
    glob_match_inner(&pi, &vi, 0, 0)
}

fn glob_match_inner(pattern: &[char], value: &[char], pi: usize, vi: usize) -> bool {
    if pi == pattern.len() && vi == value.len() {
        return true;
    }
    if pi == pattern.len() {
        return false;
    }
    if pattern[pi] == '*' {
        // Match zero or more characters
        for i in vi..=value.len() {
            if glob_match_inner(pattern, value, pi + 1, i) {
                return true;
            }
        }
        return false;
    }
    if vi == value.len() {
        return false;
    }
    if pattern[pi] == '?' || pattern[pi] == value[vi] {
        return glob_match_inner(pattern, value, pi + 1, vi + 1);
    }
    false
}
