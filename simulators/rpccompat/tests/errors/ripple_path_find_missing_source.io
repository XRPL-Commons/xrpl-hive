// Verify ripple_path_find returns error when source_account is missing
>> {"method":"ripple_path_find","params":[{"destination_account":"rfkE1aSy9G8Upk4JssnwBxhEv5p4mn2KTy","destination_amount":"1000000"}]}
<< {"result":{"error":"srcActMissing","status":"error"}}
