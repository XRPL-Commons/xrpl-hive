// Verify account_info for non-existent account returns actNotFound
>> {"method":"account_info","params":[{"account":"rPMh7Pi9ct699iZUTWz6CFkakUy5JNb6FG","ledger_index":"current"}]}
<< {"result":{"error":"actNotFound","status":"error"}}
