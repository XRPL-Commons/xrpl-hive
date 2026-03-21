// Verify account_info without required account param returns error
// speconly: true
>> {"method":"account_info","params":[{}]}
<< {"result":{"status":"error"}}
