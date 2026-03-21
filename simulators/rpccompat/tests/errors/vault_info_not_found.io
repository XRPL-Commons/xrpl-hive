// Verify vault_info returns error when vault does not exist
>> {"method":"vault_info","params":[{"vault_id":"E08D6E9754025BA2534A78707605E0601F03ACE063687A0CA1BDDACFCD1698C7","ledger_index":"current"}]}
<< {"result":{"status":"error"}}
