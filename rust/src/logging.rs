use std::io::Write;

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

    fn write(&self, level: &str, msg: &str, fields: &[(&str, &str)]) {
        let mut map = serde_json::Map::new();
        map.insert("ts".into(), serde_json::Value::String(chrono_now()));
        map.insert("level".into(), serde_json::Value::String(level.to_string()));
        map.insert("msg".into(), serde_json::Value::String(msg.to_string()));
        map.insert("module".into(), serde_json::Value::String(self.module.clone()));
        for (k, v) in fields {
            map.insert((*k).to_string(), serde_json::Value::String((*v).to_string()));
        }
        if let Ok(json) = serde_json::to_string(&serde_json::Value::Object(map)) {
            let _ = writeln!(std::io::stderr(), "{}", json);
        }
    }

    pub fn debug(&self, msg: &str) {
        self.write("DEBUG", msg, &[]);
    }
    pub fn info(&self, msg: &str) {
        self.write("INFO", msg, &[]);
    }
    pub fn warn(&self, msg: &str) {
        self.write("WARNING", msg, &[]);
    }
    pub fn error(&self, msg: &str) {
        self.write("ERROR", msg, &[]);
    }

    pub fn debug_with(&self, msg: &str, fields: &[(&str, &str)]) {
        self.write("DEBUG", msg, fields);
    }
    pub fn info_with(&self, msg: &str, fields: &[(&str, &str)]) {
        self.write("INFO", msg, fields);
    }
    pub fn warn_with(&self, msg: &str, fields: &[(&str, &str)]) {
        self.write("WARNING", msg, fields);
    }
    pub fn error_with(&self, msg: &str, fields: &[(&str, &str)]) {
        self.write("ERROR", msg, fields);
    }
}

#[derive(Clone)]
pub struct MinLevelLogger {
    ndjson: NdjsonLogger,
}

impl MinLevelLogger {
    pub fn new(module: &str) -> Self {
        Self {
            ndjson: NdjsonLogger::new(module),
        }
    }

    pub fn debug(&self, _msg: &str) {}
    pub fn info(&self, _msg: &str) {}
    pub fn warn(&self, msg: &str) {
        self.ndjson.warn(msg);
    }
    pub fn error(&self, msg: &str) {
        self.ndjson.error(msg);
    }

    pub fn debug_with(&self, _msg: &str, _fields: &[(&str, &str)]) {}
    pub fn info_with(&self, _msg: &str, _fields: &[(&str, &str)]) {}
    pub fn warn_with(&self, msg: &str, fields: &[(&str, &str)]) {
        self.ndjson.warn_with(msg, fields);
    }
    pub fn error_with(&self, msg: &str, fields: &[(&str, &str)]) {
        self.ndjson.error_with(msg, fields);
    }
}

#[derive(Clone)]
pub enum WgrokLogger {
    Ndjson(NdjsonLogger),
    MinLevel(MinLevelLogger),
}

impl WgrokLogger {
    pub fn debug(&self, msg: &str) {
        match self {
            Self::Ndjson(l) => l.debug(msg),
            Self::MinLevel(l) => l.debug(msg),
        }
    }
    pub fn info(&self, msg: &str) {
        match self {
            Self::Ndjson(l) => l.info(msg),
            Self::MinLevel(l) => l.info(msg),
        }
    }
    pub fn warn(&self, msg: &str) {
        match self {
            Self::Ndjson(l) => l.warn(msg),
            Self::MinLevel(l) => l.warn(msg),
        }
    }
    pub fn error(&self, msg: &str) {
        match self {
            Self::Ndjson(l) => l.error(msg),
            Self::MinLevel(l) => l.error(msg),
        }
    }

    pub fn debug_with(&self, msg: &str, fields: &[(&str, &str)]) {
        match self {
            Self::Ndjson(l) => l.debug_with(msg, fields),
            Self::MinLevel(l) => l.debug_with(msg, fields),
        }
    }
    pub fn info_with(&self, msg: &str, fields: &[(&str, &str)]) {
        match self {
            Self::Ndjson(l) => l.info_with(msg, fields),
            Self::MinLevel(l) => l.info_with(msg, fields),
        }
    }
    pub fn warn_with(&self, msg: &str, fields: &[(&str, &str)]) {
        match self {
            Self::Ndjson(l) => l.warn_with(msg, fields),
            Self::MinLevel(l) => l.warn_with(msg, fields),
        }
    }
    pub fn error_with(&self, msg: &str, fields: &[(&str, &str)]) {
        match self {
            Self::Ndjson(l) => l.error_with(msg, fields),
            Self::MinLevel(l) => l.error_with(msg, fields),
        }
    }
}

pub fn get_logger(debug: bool, module: &str) -> WgrokLogger {
    if debug {
        WgrokLogger::Ndjson(NdjsonLogger::new(module))
    } else {
        WgrokLogger::MinLevel(MinLevelLogger::new(module))
    }
}

fn chrono_now() -> String {
    // Simple UTC timestamp without chrono dependency
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default();
    format!("{}.{:09}Z", now.as_secs(), now.subsec_nanos())
}
