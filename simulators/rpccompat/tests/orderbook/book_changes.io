// Verify book_changes returns structure for current ledger
// speconly: true
>> {"method":"book_changes","params":[{"ledger_index":"current"}]}
<< {"result":{"type":"bookChanges","ledger_index":"...","changes":[],"status":"success"}}
