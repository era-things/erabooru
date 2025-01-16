use axum::extract::State;
use axum::routing::{delete, get, post};
use axum::{Json, Router};

use crate::model::{ItemMetadata, ItemMetadataRequest, ModelController};
use crate::error::{Error, Result};

pub fn routes(mc: ModelController) -> Router{
    Router::new()
        .route("/items", get(get_items))
        .route("/add_item", post(create_item).get(get_items))
        .with_state(mc)
}

async fn create_item(
    State(mc): State<ModelController>, 
    Json(item_fc): Json<ItemMetadataRequest>
) -> Result<Json<ItemMetadata>> {
    println!("->> {:<12} - create_item", "HANDLER");

    let item = mc.add_item(item_fc).await?;
    Ok(Json(item))
}

async fn get_items(State(mc): State<ModelController>) -> Result<Json<Vec<ItemMetadata>>> {
    println!("->> {:<12} - get_items", "HANDLER");

    let items = mc.get_items().await?;
    Ok(Json(items))
}