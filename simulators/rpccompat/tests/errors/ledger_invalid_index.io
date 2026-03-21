// Verify ledger returns error for non-existent ledger index
>> {"method":"ledger","params":[{"ledger_index":999999999}]}
<< {"result":{"error":"lgrNotFound","status":"error"}}
