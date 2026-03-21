// Verify account_channels returns empty list for genesis account
// speconly: true
>> {"method":"account_channels","params":[{"account":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh","ledger_index":"current"}]}
<< {"result":{"account":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh","channels":[],"status":"success"}}
