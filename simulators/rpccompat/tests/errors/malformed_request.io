// Verify request without method field returns error
// speconly: true
>> {"method":"","params":[{}]}
<< {"result":{"status":"error"}}
