// Verify get_aggregate_price returns error when oracles are missing
>> {"method":"get_aggregate_price","params":[{"base_asset":"XRP","quote_asset":"USD"}]}
<< {"result":{"error":"invalidParams","status":"error"}}
