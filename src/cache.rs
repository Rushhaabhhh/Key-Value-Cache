use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use linked_hash_map::LinkedHashMap;
use thiserror::Error;
use sysinfo::{System, SystemExt};
use tokio::time::{sleep, Duration};

#[derive(Debug, Error)]
pub enum CacheError {
    #[error("key not found")]
    KeyNotFound,
    #[error("key exceeds maximum length")]
    KeyTooLarge,
    #[error("value exceeds maximum length")]
    ValueTooLarge,
}

struct CacheInner {
    data: LinkedHashMap<String, String>,
    max_key_size: usize,
    max_val_size: usize,
}

#[derive(Clone)]
pub struct Cache {
    inner: Arc<RwLock<CacheInner>>,
}

impl Cache {
    pub fn new(max_key_size: usize, max_value_size: usize) -> Self {
        Cache {
            inner: Arc::new(RwLock::new(CacheInner {
                data: LinkedHashMap::new(),
                max_key_size,
                max_val_size: max_value_size,
            })),
        }
    }

    pub fn put(&self, key: String, value: String) -> Result<(), CacheError> {
        if key.len() > self.inner.read().unwrap().max_key_size {
            return Err(CacheError::KeyTooLarge);
        }
        
        if value.len() > self.inner.read().unwrap().max_val_size {
            return Err(CacheError::ValueTooLarge);
        }
        
        let mut cache = self.inner.write().unwrap();
        
        // Remove and reinsert to update LRU order
        cache.data.remove(&key);
        cache.data.insert(key, value);
        
        Ok(())
    }

    pub fn get(&self, key: &str) -> Result<String, CacheError> {
        let mut cache = self.inner.write().unwrap();
        
        if let Some(value) = cache.data.get(key) {
            // Update LRU order by removing and reinserting
            let value = value.clone();
            cache.data.remove(key);
            cache.data.insert(key.to_string(), value.clone());
            Ok(value)
        } else {
            Err(CacheError::KeyNotFound)
        }
    }

    pub async fn monitor_memory_usage(&self) {
        log::info!("Monitoring memory usage...");
        let mut sys = System::new_all();
        
        loop {
            sys.refresh_all();
            
            let total_memory = sys.total_memory();
            let used_memory = sys.used_memory();
            let threshold = total_memory * 70 / 100; // 70% threshold
            
            let mem_usage_mb = used_memory / 1024;
            let threshold_mb = threshold / 1024;
            
            if used_memory > threshold {
                log::warn!(
                    "Memory usage Critical: {} MB used (threshold: {} MB).",
                    mem_usage_mb,
                    threshold_mb
                );
                self.evict(threshold).await;
            }
            
            sleep(Duration::from_secs(1)).await;
        }
    }

    async fn evict(&self, threshold: u64) {
        let mut batch_size = 5;
        let mut sys = System::new_all();
        
        loop {
            sys.refresh_memory();
            
            if sys.used_memory() <= threshold / 2 {
                log::info!(
                    "Memory reduced to {} MB (below target {} MB), stopping eviction",
                    sys.used_memory() / 1024,
                    threshold / 2 / 1024
                );
                break;
            }
            
            let removed = {
                let mut cache = self.inner.write().unwrap();
                if cache.data.is_empty() {
                    log::info!("Cache empty, cannot evict further");
                    0
                } else {
                    let mut removed = 0;
                    while removed < batch_size && !cache.data.is_empty() {
                        // LinkedHashMap doesn't have a direct way to get the oldest entry
                        // We get the first key (oldest one) by iterating
                        if let Some(oldest_key) = cache.data.keys().next().cloned() {
                            cache.data.remove(&oldest_key);
                            removed += 1;
                        } else {
                            break;
                        }
                    }
                    removed
                }
            };
            
            if removed == 0 {
                break;
            }
            
            batch_size = std::cmp::min(batch_size * 2, 1000);
            sleep(Duration::from_millis(50)).await;
        }
    }
}
