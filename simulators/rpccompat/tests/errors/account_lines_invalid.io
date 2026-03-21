// Verify account_lines returns error for invalid account format
>> {"method":"account_lines","params":[{"account":"INVALID","ledger_index":"current"}]}
<< {"result":{"error":"actMalformed","status":"error"}}
