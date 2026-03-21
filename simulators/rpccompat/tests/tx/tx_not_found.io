// Verify tx for non-existent hash returns txnNotFound
>> {"method":"tx","params":[{"transaction":"0000000000000000000000000000000000000000000000000000000000000000"}]}
<< {"result":{"error":"txnNotFound","status":"error"}}
