// Verify account_info for non-existent account returns actNotFound
// Use a valid but unfunded address (generated via wallet_propose)
>> {"method":"account_info","params":[{"account":"rfkE1aSy9G8Upk4JssnwBxhEv5p4mn2KTy","ledger_index":"current"}]}
<< {"result":{"error":"actNotFound","status":"error"}}
