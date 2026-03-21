// Verify wallet_propose with ed25519 key type
// speconly: true
>> {"method":"wallet_propose","params":[{"key_type":"ed25519"}]}
<< {"result":{"account_id":"...","key_type":"ed25519","master_seed":"...","public_key":"...","status":"success"}}
