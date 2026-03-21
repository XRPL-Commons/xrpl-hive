// Verify ledger_entry can fetch the genesis account
// speconly: true
>> {"method":"ledger_entry","params":[{"account_root":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh","ledger_index":"validated"}]}
<< {"result":{"index":"...","node":{"Account":"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh","Balance":"...","LedgerEntryType":"AccountRoot"},"status":"success"}}
