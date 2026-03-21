// Verify ledger_accept advances the ledger in standalone mode
// speconly: true
>> {"method":"ledger_accept","params":[{}]}
<< {"result":{"ledger_current_index":"...","status":"success"}}
