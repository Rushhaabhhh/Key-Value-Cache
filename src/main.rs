use std::env;
use std::net::SocketAddr;
use std::sync::Arc;
use tokio::signal;

mod api;
mod cache;
mod config;
mod handlers;
mod models;
#[tokio::main]
async fn main() {
    // Set up logging
    env_logger::init();

    let cfg = config::load_config();
    log::info!(
        "Starting remoteDictionary with port={}, maxKeySize={}, maxValueSize={}",
        cfg.port,
        cfg.max_key_size,
        cfg.max_value_size
    );

    let server = api::new_server(cfg);

    // Start the server in a separate task
    let server_handle = tokio::spawn(async move {
        if let Err(e) = server.start().await {
            log::error!("Failed to start server: {}", e);
            std::process::exit(1);
        }
    });

    // Wait for shutdown signal
    signal::ctrl_c().await.expect("Failed to listen for control-C");
    log::info!("Shutting down server...");

    // Wait for server to finish
    server_handle.abort();
    log::info!("Server exiting");
}