// Verify account_info returns error for invalid account format
>> {"method":"account_info","params":[{"account":"not_a_valid_address","ledger_index":"current"}]}
<< {"result":{"error":"actMalformed","status":"error"}}
