// Verify ledger_header returns header fields for current ledger
// speconly: true
>> {"method":"ledger_header","params":[{"ledger_index":"current"}]}
<< {"result":{"ledger":{"ledger_index":"..."},"status":"success"}}
