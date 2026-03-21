// Verify channel_authorize returns error when channel_id is missing
>> {"method":"channel_authorize","params":[{"amount":"1000000","secret":"snoPBrXtMeMyMHUVTgbuqAfg1SUTb"}]}
<< {"result":{"error":"invalidParams","status":"error"}}
