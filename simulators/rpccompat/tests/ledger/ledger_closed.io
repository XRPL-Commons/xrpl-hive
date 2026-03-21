// Verify ledger_closed response structure
// Note: standalone nodes without peers return noNetwork
// speconly: true
>> {"method":"ledger_closed","params":[{}]}
<< {"result":{"status":"..."}}
