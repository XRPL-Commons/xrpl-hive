// Verify submit_multisigned returns error when tx_json has no Signers
// speconly: true
>> {"method":"submit_multisigned","params":[{"tx_json":{"TransactionType":"Payment","Account":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh","Destination":"rfkE1aSy9G8Upk4JssnwBxhEv5p4mn2KTy","Amount":"1000000"}}]}
<< {"result":{"status":"error"}}
