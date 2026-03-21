// Verify ledger_entry returns error when no selector field is provided
>> {"method":"ledger_entry","params":[{"ledger_index":"current"}]}
<< {"result":{"error":"unknownOption","status":"error"}}
