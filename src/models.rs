use serde::{Deserialize, Serialize};

#[derive(Debug, Deserialize)]
pub struct PutRequest {
    pub key: String,
    pub value: String,
}

#[derive(Debug, Serialize)]
pub struct PutResponse {
    pub status: String,
    pub message: String,
}

#[derive(Debug, Serialize)]
pub struct GetResponse {
    pub status: String,
    pub key: String,
    pub value: String,
    pub message: String,
}

#[derive(Debug, Serialize)]
pub struct ErrorResponse {
    pub status: String,
    pub message: String,
}