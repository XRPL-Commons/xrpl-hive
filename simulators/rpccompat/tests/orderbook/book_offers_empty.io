// Verify book_offers returns empty for non-existent order book
// speconly: true
>> {"method":"book_offers","params":[{"taker_pays":{"currency":"XRP"},"taker_gets":{"currency":"USD","issuer":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"},"ledger_index":"current"}]}
<< {"result":{"offers":[],"status":"success"}}
