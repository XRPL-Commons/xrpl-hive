// Verify server_state returns required machine-readable fields
// speconly: true
>> {"method":"server_state","params":[{}]}
<< {"result":{"state":{"build_version":"...","server_state":"...","complete_ledgers":"..."},"status":"success"}}
