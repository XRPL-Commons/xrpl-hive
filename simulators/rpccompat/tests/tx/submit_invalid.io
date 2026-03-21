// Verify submit with invalid tx_blob returns error
// speconly: true
>> {"method":"submit","params":[{"tx_blob":"DEADBEEF"}]}
<< {"result":{"status":"error"}}
