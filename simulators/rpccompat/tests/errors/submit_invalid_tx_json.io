// Verify submit returns error for tx_json missing TransactionType
// speconly: true
>> {"method":"submit","params":[{"secret":"snoPBrXtMeMyMHUVTgbuqAfg1SUTb","tx_json":{"Account":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"}}]}
<< {"result":{"status":"error"}}
