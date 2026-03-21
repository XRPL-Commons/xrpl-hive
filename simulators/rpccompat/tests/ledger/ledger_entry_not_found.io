// Verify ledger_entry returns error for non-existent account root
>> {"method":"ledger_entry","params":[{"account_root":"rfkE1aSy9G8Upk4JssnwBxhEv5p4mn2KTy","ledger_index":"current"}]}
<< {"result":{"error":"entryNotFound","status":"error"}}
