// Verify tx returns error for invalid hash format
>> {"method":"tx","params":[{"transaction":"NOT_A_VALID_HASH"}]}
<< {"result":{"error":"notImpl","status":"error"}}
