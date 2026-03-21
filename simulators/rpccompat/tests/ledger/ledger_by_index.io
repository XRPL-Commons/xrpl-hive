// Verify ledger returns genesis ledger structure
// speconly: true
>> {"method":"ledger","params":[{"ledger_index":1}]}
<< {"result":{"ledger":{"ledger_hash":"...","ledger_index":"..."},"status":"success"}}
