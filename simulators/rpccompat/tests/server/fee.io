// Verify fee returns fee information
// speconly: true
>> {"method":"fee","params":[{}]}
<< {"result":{"current_ledger_size":"...","current_queue_size":"...","drops":{"base_fee":"...","median_fee":"...","minimum_fee":"...","open_ledger_fee":"..."},"expected_ledger_size":"...","ledger_current_index":"...","status":"success"}}
