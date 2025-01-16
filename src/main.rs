#![allow(unused)]

use std::net::SocketAddr;
use axum::{response::{Html, IntoResponse}, routing::get, Router};
use model::ModelController;
use crate::error::{Error, Result};

mod model;
mod error;
mod web;

#[tokio::main] 
async fn main() -> Result<()> {
    let mc = ModelController::new().await?;

    let routes_hello = Router::new()
    .route("/",get(handler_hello))
    .nest("/api", web::routes_items::routes(mc.clone()));

    let addr = SocketAddr::from(([127, 0, 0, 1], 8080));
    println!("Listening on http://{}", addr);
    
    axum_server::bind(addr)
        .serve(routes_hello.into_make_service())
        .await
        .unwrap();

    Ok(())
}

async fn handler_hello() -> impl IntoResponse {
    println!("->> {:<12} - handler_hello", "HANDLER");
    Html("Hello, <strong>World!</strong>")
}
