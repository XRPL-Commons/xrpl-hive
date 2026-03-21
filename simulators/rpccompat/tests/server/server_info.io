// Verify server_info returns required fields
// speconly: true
>> {"method":"server_info","params":[{}]}
<< {"result":{"info":{"build_version":"...","server_state":"...","complete_ledgers":"..."},"status":"success"}}
