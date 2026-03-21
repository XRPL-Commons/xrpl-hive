// Verify wallet_propose with passphrase returns deterministic keypair
// speconly: true
>> {"method":"wallet_propose","params":[{"passphrase":"masterpassphrase"}]}
<< {"result":{"account_id":"...","key_type":"...","master_key":"...","master_seed":"...","master_seed_hex":"...","public_key":"...","public_key_hex":"...","status":"success"}}
