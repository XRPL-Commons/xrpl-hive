// Verify json proxy method forwards to the specified method
// speconly: true
>> {"method":"json","params":[{"method":"ping"}]}
<< {"result":{"status":"success"}}
