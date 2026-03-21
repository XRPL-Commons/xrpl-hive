// Verify ledger_closed returns the most recent closed ledger
// speconly: true
>> {"method":"ledger_closed","params":[{}]}
<< {"result":{"ledger_hash":"...","ledger_index":"...","status":"success"}}
