// Verify stop returns the stopping message
// speconly: true
// NOTE: This test may cause the node to shut down. Run with caution.
>> {"method":"stop","params":[{}]}
<< {"result":{"message":"...","status":"success"}}
