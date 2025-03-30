use std::convert::Infallible;
use std::net::SocketAddr;
use std::sync::Arc;
use std::time::Duration;
use chrono::Utc;
use hyper::service::{make_service_fn, service_fn};
use hyper::{Body, Method, Request, Response, Server, StatusCode};
use serde_json::json;
use tokio::time::timeout;
use tower_http::cors::{Any, CorsLayer};

use crate::cache::Cache;
use crate::config::Config;
use crate::handlers::Handler;

pub struct ApiServer {
    handler: Handler,
    config: Config,
}

pub fn new_server(cfg: Config) -> ApiServer {
    let cache = Arc::new(Cache::new(cfg.max_key_size, cfg.max_value_size));
    
    // Start memory monitor in a separate task
    let cache_clone = Arc::clone(&cache);
    tokio::spawn(async move {
        cache_clone.monitor_memory_usage().await;
    });
    
    let handler = Handler::new(cache);
    
    ApiServer {
        handler,
        config: cfg,
    }
}

impl ApiServer {
    pub async fn start(&self) -> Result<(), hyper::Error> {
        let handler = self.handler.clone();
        
        let cors = CorsLayer::new()
            .allow_origin(Any)
            .allow_methods([Method::GET, Method::POST, Method::OPTIONS])
            .allow_headers(Any)
            .allow_credentials(true);
        
        // Define service to handle incoming requests
        let make_svc = make_service_fn(move |_conn| {
            let handler = handler.clone();
            async move {
                Ok::<_, Infallible>(service_fn(move |req| {
                    let handler = handler.clone();
                    async move {
                        match (req.method(), req.uri().path()) {
                            (&Method::POST, "/put") => handler.put_handler(req).await,
                            (&Method::GET, "/get") => handler.get_handler(req).await,
                            (_, "/") => {
                                let now = Utc::now().to_string();
                                let response = json!({
                                    "status": "healthy",
                                    "time": now,
                                });
                                
                                let json_response = serde_json::to_string(&response)
                                    .unwrap_or_else(|_| String::from(r#"{"status":"error"}"#));
                                    
                                let res = Response::builder()
                                    .status(StatusCode::OK)
                                    .header("Content-Type", "application/json")
                                    .body(Body::from(json_response))
                                    .unwrap();
                                    
                                Ok(res)
                            },
                            _ => {
                                let res = Response::builder()
                                    .status(StatusCode::NOT_FOUND)
                                    .body(Body::from("Not Found"))
                                    .unwrap();
                                Ok(res)
                            }
                        }
                    }
                }))
            }
        });
        
        let addr: SocketAddr = format!("0.0.0.0:{}", self.config.port)
            .parse()
            .expect("Failed to parse server address");
            
        log::info!("Starting server on port {}", self.config.port);
        
        let server = Server::bind(&addr)
            .serve(make_svc)
            .with_graceful_shutdown(async {
                tokio::signal::ctrl_c().await.ok();
            });
            
        server.await?;
        
        Ok(())
    }
}