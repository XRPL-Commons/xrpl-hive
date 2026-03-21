// Verify path_find returns notSupported over HTTP (WebSocket only)
// speconly: true
>> {"method":"path_find","params":[{"subcommand":"create","source_account":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh","destination_account":"rfkE1aSy9G8Upk4JssnwBxhEv5p4mn2KTy","destination_amount":"1000000"}]}
<< {"result":{"status":"..."}}
