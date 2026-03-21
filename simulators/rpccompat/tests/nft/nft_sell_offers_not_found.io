// Verify nft_sell_offers returns error for non-existent NFT ID
>> {"method":"nft_sell_offers","params":[{"nft_id":"00080000B4F4AFC5FBBED8B0E1F5E5E0F4A3B2C1D0E9F8A7B6C5D4E300000001"}]}
<< {"result":{"error":"objectNotFound","status":"error"}}
