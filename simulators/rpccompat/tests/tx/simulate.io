// Verify simulate returns simulated transaction result without committing
// speconly: true
>> {"method":"simulate","params":[{"tx_json":{"TransactionType":"Payment","Account":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh","Destination":"rfkE1aSy9G8Upk4JssnwBxhEv5p4mn2KTy","Amount":"1000000","Fee":"12","Sequence":1}}]}
<< {"result":{"engine_result":"...","engine_result_code":"...","engine_result_message":"...","status":"success"}}
