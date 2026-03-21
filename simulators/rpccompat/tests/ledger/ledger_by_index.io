// Verify ledger returns the current open ledger structure
// speconly: true
>> {"method":"ledger","params":[{"ledger_index":"current"}]}
<< {"result":{"ledger":{"ledger_hash":"...","ledger_index":"..."},"status":"success"}}
