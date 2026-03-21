// Verify ledger_current returns current ledger index
// Note: standalone nodes without peers may return noNetwork
// speconly: true
>> {"method":"ledger_current","params":[{}]}
<< {"result":{"status":"..."}}
