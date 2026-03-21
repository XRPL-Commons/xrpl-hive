// Verify account_tx returns error for invalid account format
>> {"method":"account_tx","params":[{"account":"INVALID"}]}
<< {"result":{"error":"actMalformed","status":"error"}}
