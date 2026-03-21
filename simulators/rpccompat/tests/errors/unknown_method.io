// Verify unknown method returns unknownCmd error
>> {"method":"nonexistent_method_xyz","params":[{}]}
<< {"result":{"error":"unknownCmd","status":"error"}}
