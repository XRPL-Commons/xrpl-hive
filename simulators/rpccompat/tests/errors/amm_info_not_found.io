// Verify amm_info returns error when AMM pool does not exist
>> {"method":"amm_info","params":[{"asset":{"currency":"USD","issuer":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"},"asset2":{"currency":"XRP"}}]}
<< {"result":{"error":"actNotFound","status":"error"}}
