use hyper::{Body, Request, Response, StatusCode};
use serde_json::json;
use std::sync::Arc;

use crate::cache::{Cache, CacheError};
use crate::models::{ErrorResponse, GetResponse, PutRequest, PutResponse};

#[derive(Clone)]
pub struct Handler {
    cache: Arc<Cache>,
}

impl Handler {
    pub fn new(cache: Arc<Cache>) -> Self {
        Handler { cache }
    }

    pub async fn put_handler(&self, req: Request<Body>) -> Result<Response<Body>, hyper::Error> {
        // Check if method is POST
        if req.method() != hyper::Method::POST {
            return Ok(self.method_not_allowed_response());
        }

        // Read the request body
        let body_bytes = hyper::body::to_bytes(req.into_body()).await?;
        
        // Parse the JSON body
        let put_req: Result<PutRequest, _> = serde_json::from_slice(&body_bytes);
        
        match put_req {
            Ok(put_req) => {
                // Check if key is empty
                if put_req.key.trim().is_empty() {
                    return Ok(self.send_error_response(
                        "Key cannot be empty",
                        StatusCode::BAD_REQUEST,
                    ));
                }
                
                // Put the key-value pair in the cache
                match self.cache.put(put_req.key, put_req.value) {
                    Ok(_) => {
                        let resp = PutResponse {
                            status: "OK".to_string(),
                            message: "Key inserted/updated successfully.".to_string(),
                        };
                        
                        self.send_json_response(&resp, StatusCode::OK)
                    }
                    Err(err) => match err {
                        CacheError::KeyTooLarge => {
                            Ok(self.send_error_response(
                                "Key exceeds maximum length (256 characters)",
                                StatusCode::BAD_REQUEST,
                            ))
                        }
                        CacheError::ValueTooLarge => {
                            Ok(self.send_error_response(
                                "Value exceeds maximum length (256 characters)",
                                StatusCode::BAD_REQUEST,
                            ))
                        }
                        _ => Ok(self.send_error_response(
                            "Internal server error",
                            StatusCode::INTERNAL_SERVER_ERROR,
                        )),
                    },
                }
            }
            Err(_) => Ok(self.send_error_response("Invalid JSON format", StatusCode::BAD_REQUEST)),
        }
    }

    pub async fn get_handler(&self, req: Request<Body>) -> Result<Response<Body>, hyper::Error> {
        // Check if method is GET
        if req.method() != hyper::Method::GET {
            return Ok(self.method_not_allowed_response());
        }

        // Parse query parameters
        let query = req.uri().query().unwrap_or("");
        let params: std::collections::HashMap<_, _> = url::form_urlencoded::parse(query.as_bytes())
            .into_owned()
            .collect();
            
        // Get the key parameter
        let key = params.get("key").map(|s| s.as_str()).unwrap_or("");
        
        if key.trim().is_empty() {
            return Ok(self.send_error_response(
                "Key parameter is missing",
                StatusCode::BAD_REQUEST,
            ));
        }
        
        match self.cache.get(key) {
            Ok(value) => {
                let resp = GetResponse {
                    status: "OK".to_string(),
                    key: key.to_string(),
                    value,
                    message: "Key retrieved successfully.".to_string(),
                };
                
                self.send_json_response(&resp, StatusCode::OK)
            }
            Err(err) => match err {
                CacheError::KeyNotFound => {
                    let resp = ErrorResponse {
                        status: "ERROR".to_string(),
                        message: "Key not found.".to_string(),
                    };
                    
                    self.send_json_response(&resp, StatusCode::OK)
                }
                _ => Ok(self.send_error_response("Internal server error", StatusCode::INTERNAL_SERVER_ERROR)),
            },
        }
    }

    fn send_json_response<T>(&self, data: &T, status: StatusCode) -> Result<Response<Body>, hyper::Error>
    where
        T: serde::Serialize,
    {
        match serde_json::to_string(data) {
            Ok(json) => {
                let res = Response::builder()
                    .status(status)
                    .header("Content-Type", "application/json")
                    .body(Body::from(json))
                    .unwrap();
                    
                Ok(res)
            }
            Err(_) => {
                let res = Response::builder()
                    .status(StatusCode::INTERNAL_SERVER_ERROR)
                    .body(Body::from("Error encoding response"))
                    .unwrap();
                    
                Ok(res)
            }
        }
    }

    fn send_error_response(&self, message: &str, status: StatusCode) -> Response<Body> {
        let resp = ErrorResponse {
            status: "ERROR".to_string(),
            message: message.to_string(),
        };
        
        match serde_json::to_string(&resp) {
            Ok(json) => Response::builder()
                .status(status)
                .header("Content-Type", "application/json")
                .body(Body::from(json))
                .unwrap(),
            Err(_) => Response::builder()
                .status(StatusCode::INTERNAL_SERVER_ERROR)
                .body(Body::from("Error encoding response"))
                .unwrap(),
        }
    }

    fn method_not_allowed_response(&self) -> Response<Body> {
        Response::builder()
            .status(StatusCode::METHOD_NOT_ALLOWED)
            .body(Body::from("Method not allowed"))
            .unwrap()
    }
}
