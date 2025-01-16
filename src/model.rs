use crate::error::{Error, Result};
use serde::{Deserialize, Serialize};
use std::sync::{Arc, Mutex};

#[derive(Clone, Debug, Serialize)]
pub struct ItemMetadata {
    pub id: u64,
    pub name: String,
    pub description: String,
}

#[derive(Deserialize)]
pub struct ItemMetadataRequest {
    pub name: String,
    pub description: String,
}

#[derive(Clone)]
pub struct ModelController{
    pub items: Arc<Mutex<Vec<ItemMetadata>>>,
}

impl ModelController { 
    pub async fn new() -> Result<Self> {
        Ok(Self {
            items: Arc::new(Mutex::new(Vec::new())),
        })
    }

    pub async fn add_item(&self, item: ItemMetadataRequest) -> Result<ItemMetadata> {
        let mut items= self.items.lock().unwrap();
        let id = items.len() as u64;
        let item = ItemMetadata {
            id,
            name: item.name,
            description: item.description,
        };
        items.push(item.clone());
        Ok(item)
    }

    pub async fn get_items(&self) -> Result<Vec<ItemMetadata>> {
        let items = self.items.lock().unwrap();
        Ok(items.clone())
    }

    //TODO: Delete item
}