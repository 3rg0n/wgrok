use serde::Serialize;
use std::io::Write;

#[derive(Serialize)]
struct LogLine<'a> {
    ts: String,
    level: &'a str,
    msg: &'a str,
    module: &'a str,
}

#[derive(Clone)]
pub struct NdjsonLogger {
    pub module: String,
}

impl NdjsonLogger {
    pub fn new(module: &str) -> Self {
        Self {
            module: module.to_string(),
        }
    }

    fn write(&self, level: &str, msg: &str) {
        let line = LogLine {
            ts: chrono_now(),
            level,
            msg,
            module: &self.module,
        };
        if let Ok(json) = serde_json::to_string(&line) {
            let _ = writeln!(std::io::stderr(), "{}", json);
        }
    }

    pub fn debug(&self, msg: &str) {
        self.write("DEBUG", msg);
    }
    pub fn info(&self, msg: &str) {
        self.write("INFO", msg);
    }
    pub fn warn(&self, msg: &str) {
        self.write("WARNING", msg);
    }
    pub fn error(&self, msg: &str) {
        self.write("ERROR", msg);
    }
}

#[derive(Clone)]
pub struct NoopLogger;

impl NoopLogger {
    pub fn debug(&self, _msg: &str) {}
    pub fn info(&self, _msg: &str) {}
    pub fn warn(&self, _msg: &str) {}
    pub fn error(&self, _msg: &str) {}
}

#[derive(Clone)]
pub enum WgrokLogger {
    Ndjson(NdjsonLogger),
    Noop(NoopLogger),
}

impl WgrokLogger {
    pub fn debug(&self, msg: &str) {
        match self {
            Self::Ndjson(l) => l.debug(msg),
            Self::Noop(l) => l.debug(msg),
        }
    }
    pub fn info(&self, msg: &str) {
        match self {
            Self::Ndjson(l) => l.info(msg),
            Self::Noop(l) => l.info(msg),
        }
    }
    pub fn warn(&self, msg: &str) {
        match self {
            Self::Ndjson(l) => l.warn(msg),
            Self::Noop(l) => l.warn(msg),
        }
    }
    pub fn error(&self, msg: &str) {
        match self {
            Self::Ndjson(l) => l.error(msg),
            Self::Noop(l) => l.error(msg),
        }
    }
}

pub fn get_logger(debug: bool, module: &str) -> WgrokLogger {
    if debug {
        WgrokLogger::Ndjson(NdjsonLogger::new(module))
    } else {
        WgrokLogger::Noop(NoopLogger)
    }
}

fn chrono_now() -> String {
    // Simple UTC timestamp without chrono dependency
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default();
    format!("{}.{:09}Z", now.as_secs(), now.subsec_nanos())
}
