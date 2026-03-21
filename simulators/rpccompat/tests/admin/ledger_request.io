// Verify ledger_request returns response (returns notSynced in standalone)
// speconly: true
>> {"method":"ledger_request","params":[{"ledger_index":1}]}
<< {"result":{"status":"..."}}
