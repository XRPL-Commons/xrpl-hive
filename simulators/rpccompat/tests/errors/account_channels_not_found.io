// Verify account_channels returns error for unfunded account
>> {"method":"account_channels","params":[{"account":"rfkE1aSy9G8Upk4JssnwBxhEv5p4mn2KTy","ledger_index":"current"}]}
<< {"result":{"error":"actNotFound","status":"error"}}
