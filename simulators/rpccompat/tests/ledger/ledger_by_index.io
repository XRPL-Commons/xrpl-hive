// Verify ledger returns a closed ledger structure
// Use ledger_index 2 (after ledger_accept has advanced past it)
// speconly: true
>> {"method":"ledger","params":[{"ledger_index":2}]}
<< {"result":{"ledger":{"ledger_hash":"...","ledger_index":"..."},"status":"success"}}
