// Verify connect returns response (may return notSynced in standalone)
// speconly: true
>> {"method":"connect","params":[{"ip":"127.0.0.1"}]}
<< {"result":{"status":"..."}}
