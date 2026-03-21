// Verify transaction_entry returns error for non-existent or unsupported lookup
// speconly: true
>> {"method":"transaction_entry","params":[{"tx_hash":"E08D6E9754025BA2534A78707605E0601F03ACE063687A0CA1BDDACFCD1698C7","ledger_index":"current"}]}
<< {"result":{"status":"error"}}
