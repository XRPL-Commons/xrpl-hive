// Verify fee response structure
// Note: standalone nodes without peers may return noNetwork
// speconly: true
>> {"method":"fee","params":[{}]}
<< {"result":{"status":"..."}}
