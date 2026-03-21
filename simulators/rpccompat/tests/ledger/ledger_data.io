// Verify ledger_data returns state entries for current ledger
// speconly: true
>> {"method":"ledger_data","params":[{"ledger_index":"current","limit":5}]}
<< {"result":{"ledger_index":"...","state":["..."],"status":"success"}}
