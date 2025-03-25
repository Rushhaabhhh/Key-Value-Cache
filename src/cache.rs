use dashmap::DashMap;
use snap::raw::{Encoder, Decoder};
use rmp_serde::{from_slice, to_vec};
use serde::{Deserialize, Serialize};
use bytes::Bytes;

#[derive(Clone)]
pub struct Cache {
    store: DashMap<String, Bytes>,
}

impl Cache {
    pub fn new() -> Self {
        Self {
            store: DashMap::new(),
        }
    }

    pub fn put(&self, key: String, value: impl Serialize) {
        let serialized = to_vec(&value).unwrap(); // MessagePack Serialization
        let compressed = Encoder::new().compress_vec(&serialized).unwrap();
        self.store.insert(key, Bytes::from(compressed)); // Zero-copy storage
    }

    pub fn get<T: for<'a> Deserialize<'a>>(&self, key: &str) -> Option<T> {
        self.store.get(key).and_then(|v| {
            let decompressed = Decoder::new().decompress_vec(&v).ok()?;
            from_slice(&decompressed).ok()
        })
    }
}
