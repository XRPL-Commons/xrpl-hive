// Verify book_offers returns error for invalid currency format
>> {"method":"book_offers","params":[{"taker_pays":{"currency":"TOOLONG123","issuer":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"},"taker_gets":{"currency":"XRP"}}]}
<< {"result":{"error":"srcCurMalformed","status":"error"}}
