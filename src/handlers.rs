use actix_web::{web, HttpResponse, Responder};
use serde::{Deserialize, Serialize};
use crate::cache::Cache;

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

pub async fn put_handler(cache: web::Data<std::sync::Arc<Cache>>, req: web::Json<PutRequest>) -> impl Responder {
    if req.key.len() > 256 || req.value.len() > 256 {
        return HttpResponse::BadRequest().json(ErrorResponse {
            status: "ERROR".to_string(),
            message: "Key or value exceeds length limit".to_string(),
        });
    }

    cache.put(req.key.clone(), req.value.clone());
    HttpResponse::Ok().json(serde_json::json!({
        "status": "OK",
        "message": "Key inserted/updated successfully."
    }))
}

pub async fn get_handler(cache: web::Data<std::sync::Arc<Cache>>, query: web::Query<std::collections::HashMap<String, String>>) -> impl Responder {
    if let Some(key) = query.get("key") {
        if let Some(value) = cache.get(key) {
            return HttpResponse::Ok().json(GetResponse {
                status: "OK".to_string(),
                key: key.clone(),
                value,
            });
        }
    }

    HttpResponse::NotFound().json(ErrorResponse {
        status: "ERROR".to_string(),
        message: "Key not found.".to_string(),
    })
}
