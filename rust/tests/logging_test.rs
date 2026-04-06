use wgrok::{get_logger, NdjsonLogger, MinLevelLogger, WgrokLogger};

#[test]
fn test_get_logger_debug_true_returns_ndjson() {
    let logger = get_logger(true, "test");
    assert!(matches!(logger, WgrokLogger::Ndjson(_)));
}

#[test]
fn test_get_logger_debug_false_returns_min_level() {
    let logger = get_logger(false, "test");
    assert!(matches!(logger, WgrokLogger::MinLevel(_)));
}

#[test]
fn test_ndjson_logger_module() {
    let logger = NdjsonLogger::new("wgrok.test");
    assert_eq!(logger.module, "wgrok.test");
}

#[test]
fn test_ndjson_logger_default_module() {
    let logger = NdjsonLogger::new("wgrok");
    assert_eq!(logger.module, "wgrok");
}

#[test]
fn test_min_level_logger_does_not_panic() {
    let logger = MinLevelLogger::new("test");
    logger.debug("x");
    logger.info("x");
    logger.warn("x");
    logger.error("x");
}
