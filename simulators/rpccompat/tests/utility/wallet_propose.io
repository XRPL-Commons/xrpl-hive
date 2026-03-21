// Verify wallet_propose returns a new keypair
// speconly: true
>> {"method":"wallet_propose","params":[{}]}
<< {"result":{"account_id":"...","key_type":"...","master_key":"...","master_seed":"...","master_seed_hex":"...","public_key":"...","public_key_hex":"...","status":"success"}}
