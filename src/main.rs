use actix_web::{web, App, HttpServer};
use cache::Cache;
use handlers::{put_handler, get_handler};
use actix_web::middleware::Logger;
use std::sync::Arc;

mod handlers;
mod cache;

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    // Set up logger (with env_logger you can control logging level externally)
    env_logger::init();


    // Create a shared cache instance (DashMap gives concurrent performance)
    let cache = Arc::new(Cache::new());

    HttpServer::new(move || {
        App::new()
            .app_data(web::Data::new(cache.clone()))
            .wrap(Logger::default())
            // If needed, you can adjust keep-alive timeouts etc.
            .route("/put", web::post().to(put_handler))
            .route("/get", web::get().to(get_handler))
    })
        .bind("0.0.0.0:7171")?
        .workers(2)                  // Use 2 worker threads – one per core on a 2–core instance.
        .backlog(512)
        .max_connections(500)
        .max_connection_rate(50)
        .run()
        .await
}
