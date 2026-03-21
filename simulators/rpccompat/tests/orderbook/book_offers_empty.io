// Verify book_offers returns empty for XRP/USD when no offers exist
// speconly: true
>> {"method":"book_offers","params":[{"taker_pays":{"currency":"XRP"},"taker_gets":{"currency":"USD","issuer":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"}}]}
<< {"result":{"offers":[],"status":"success"}}
