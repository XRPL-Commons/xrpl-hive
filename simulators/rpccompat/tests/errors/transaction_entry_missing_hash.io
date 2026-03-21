// Verify transaction_entry returns error when tx_hash is missing
>> {"method":"transaction_entry","params":[{"ledger_index":"current"}]}
<< {"result":{"error":"fieldNotFoundTransaction","status":"error"}}
