// Verify book_offers returns error when taker_pays is missing
>> {"method":"book_offers","params":[{"taker_gets":{"currency":"XRP"}}]}
<< {"result":{"error":"invalidParams","status":"error"}}
