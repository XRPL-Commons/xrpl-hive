// Verify sign returns error when no secret is provided
>> {"method":"sign","params":[{"tx_json":{"TransactionType":"Payment","Account":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh","Destination":"rfkE1aSy9G8Upk4JssnwBxhEv5p4mn2KTy","Amount":"1000000"}}]}
<< {"result":{"error":"invalidParams","status":"error"}}
