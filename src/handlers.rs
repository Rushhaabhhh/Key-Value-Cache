use actix_web::{web, HttpResponse, Responder};
use serde::{Deserialize, Serialize};
use crate::cache::Cache;
use std::collections::HashMap;

#[derive(Deserialize)]
pub struct PutRequest {
    key: String,
    value: String,
}

#[derive(Serialize)]
struct GetResponse {
    status: String,
    key: String,
    value: String,
}

#[derive(Serialize)]
struct ErrorResponse {
    status: String,
    message: String,
}

// Offload the (de)compression work using spawn_blocking so that if it’s CPU–bound it won’t block the async reactor.
pub async fn put_handler(cache: web::Data<std::sync::Arc<Cache>>, req: web::Json<PutRequest>) -> impl Responder {
    // Limit key/value length to avoid overflow or excessive processing.
    if req.key.len() > 256 || req.value.len() > 256 {
        return HttpResponse::BadRequest().json(ErrorResponse {
            status: "ERROR".to_string(),
            message: "Key or value exceeds length limit".to_string(),
        });
    }


    // Clone key and value for the blocking task.
    let key = req.key.clone();
    let value = req.value.clone();
    let cache = cache.get_ref().clone();

    // Spawn a blocking task for the compression/serialization work.
    let result = actix_web::web::block(move || {
        cache.put(key, value);
        Ok::<(), ()>(())
    }).await;

    match result {
        Ok(_) => HttpResponse::Ok().json(serde_json::json!({
        "status": "OK",
        "message": "Key inserted/updated successfully."
    })),
        Err(e) => {
            log::error!("Error in put_handler: {:?}", e);
            HttpResponse::InternalServerError().json(ErrorResponse {
                status: "ERROR".to_string(),
                message: "Internal processing error".to_string(),
            })
        }
    }
}

pub async fn get_handler(cache: web::Data<std::sync::Arc<Cache>>, query: web::Query<HashMap<String, String>>) -> impl Responder {
    if let Some(key) = query.get("key") {
        let cache = cache.get_ref().clone();
        let key = key.clone();
        let key_for_response = key.clone();
        // Offload potential CPU–bound decompression/deserialization work.
        let result = actix_web::web::block(move || cache.get(&key)).await;


        if let Ok(Some(value)) = result {
            return HttpResponse::Ok().json(GetResponse {
                status: "OK".to_string(),
                key: key_for_response,
                value,
            });
        }
    }

    HttpResponse::NotFound().json(ErrorResponse {
        status: "ERROR".to_string(),
        message: "Key not found.".to_string(),
    })
}