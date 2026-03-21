// Verify nft_sell_offers returns error when nft_id is missing
>> {"method":"nft_sell_offers","params":[{}]}
<< {"result":{"error":"invalidParams","status":"error"}}
