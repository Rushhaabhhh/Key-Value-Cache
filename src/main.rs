mod handlers;
mod cache;

use actix_web::{web, App, HttpServer};
use cache::Cache;
use handlers::{put_handler, get_handler};
use actix_web::middleware::Logger;
use std::sync::Arc;

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    env_logger::init();
    let cache = Arc::new(Cache::new());

    HttpServer::new(move || {
        App::new()
            .app_data(web::Data::new(cache.clone())) // Store cache in Arc
            .wrap(Logger::default())
            .route("/put", web::post().to(put_handler))
            .route("/get", web::get().to(get_handler))
    })
        .bind("0.0.0.0:7171")?
        .workers(2)
        .backlog(512)
        .max_connections(500)
        .max_connection_rate(50)
        .run()
        .await
}
