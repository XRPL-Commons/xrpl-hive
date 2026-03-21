// Verify wallet_propose with secp256k1 key type
// speconly: true
>> {"method":"wallet_propose","params":[{"key_type":"secp256k1"}]}
<< {"result":{"account_id":"...","key_type":"secp256k1","master_seed":"...","public_key":"...","status":"success"}}
