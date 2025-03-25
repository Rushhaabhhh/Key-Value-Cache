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


    // Synchronously (de)serialize and (de)compress.
    // (It is wrapped into spawn_blocking in the handlers so that the async reactor is not blocked.)
    pub fn put(&self, key: String, value: impl Serialize) {
        // MessagePack serialization – you may consider using other faster serializers if needed.
        let serialized = to_vec(&value).expect("Serialization error");
        // Compress using snap. If desired, adjust compression parameters or swap out for a faster method.
        let compressed = Encoder::new().compress_vec(&serialized).expect("Compression error");
        // Insert using zero–copy Bytes.
        self.store.insert(key, Bytes::from(compressed));
    }

    pub fn get<T: for<'a> Deserialize<'a>>(&self, key: &str) -> Option<T> {
        self.store.get(key).and_then(|v| {
            // Decompress data
            let decompressed = Decoder::new().decompress_vec(&v).ok()?;
            // Deserialize MessagePack bytes into T.
            from_slice(&decompressed).ok()
        })
    }
}