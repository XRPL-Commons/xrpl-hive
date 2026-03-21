// Verify tx for non-existent hash returns an error
// Note: may return txnNotFound or noNetwork depending on node state
// speconly: true
>> {"method":"tx","params":[{"transaction":"0000000000000000000000000000000000000000000000000000000000000000"}]}
<< {"result":{"status":"error"}}
