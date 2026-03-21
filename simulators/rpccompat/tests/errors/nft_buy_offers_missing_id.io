// Verify nft_buy_offers returns error when nft_id is missing
>> {"method":"nft_buy_offers","params":[{}]}
<< {"result":{"error":"invalidParams","status":"error"}}
