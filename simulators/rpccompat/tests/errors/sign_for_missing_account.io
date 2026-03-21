// Verify sign_for returns error when account field is missing
>> {"method":"sign_for","params":[{"secret":"snoPBrXtMeMyMHUVTgbuqAfg1SUTb","tx_json":{"TransactionType":"Payment","Account":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh","Destination":"rfkE1aSy9G8Upk4JssnwBxhEv5p4mn2KTy","Amount":"1000000"}}]}
<< {"result":{"error":"invalidParams","status":"error"}}
