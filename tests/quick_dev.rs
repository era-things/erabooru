use anyhow::Result;
use serde_json::json;

#[tokio::test]
async fn quick_dev() -> Result<()> {
    
    let hc = httpc_test::new_client("http://localhost:8080")?;

    hc.do_post(
        "/api/add_item",
        json!( {
            "name": "item1",
            "description": "item1 description"
        }),
    ).await?;

    let req_get_items = hc.do_get("/api/items");
    req_get_items.await?.print().await?;


    Ok(())
}