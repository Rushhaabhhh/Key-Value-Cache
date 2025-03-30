use dotenv::dotenv;
use std::env;

pub struct Config {
    pub port: String,
    pub max_key_size: usize,
    pub max_value_size: usize,
}

pub fn load_config() -> Config {
    dotenv().ok();
    
    let port = env::var("PORT").unwrap_or_else(|_| "7171".to_string());
    
    let max_key_size = env::var("MAX_KEY_SIZE")
        .ok()
        .and_then(|s| s.parse::<usize>().ok())
        .unwrap_or(256);
        
    let max_key_size = if max_key_size > 256 {
        log::warn!("Warning: MAX_KEY_SIZE exceeds limit of 256, using 256");
        256
    } else {
        max_key_size
    };
    
    let max_value_size = env::var("MAX_VALUE_SIZE")
        .ok()
        .and_then(|s| s.parse::<usize>().ok())
        .unwrap_or(256);
        
    let max_value_size = if max_value_size > 256 {
        log::warn!("Warning: MAX_VALUE_SIZE exceeds limit of 256, using 256");
        256
    } else {
        max_value_size
    };
    
    Config {
        port,
        max_key_size,
        max_value_size,
    }
}
