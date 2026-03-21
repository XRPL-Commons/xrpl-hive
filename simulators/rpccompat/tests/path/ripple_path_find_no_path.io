// Verify ripple_path_find returns error when destination is unfunded
>> {"method":"ripple_path_find","params":[{"source_account":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh","destination_account":"rfkE1aSy9G8Upk4JssnwBxhEv5p4mn2KTy","destination_amount":{"currency":"USD","issuer":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh","value":"10"}}]}
<< {"result":{"error":"actNotFound","status":"error"}}
