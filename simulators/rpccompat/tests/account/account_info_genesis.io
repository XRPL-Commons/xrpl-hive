// Verify account_info returns genesis account data
// speconly: true
>> {"method":"account_info","params":[{"account":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh","ledger_index":"current"}]}
<< {"result":{"account_data":{"Account":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh","Balance":"...","LedgerEntryType":"AccountRoot","Sequence":"..."},"status":"success"}}
