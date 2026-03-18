@'
-- 0001_create_customers.down.sql
DROP TABLE IF EXISTS customers;
'@ | Set-Content -Encoding UTF8 `
"$env:USERPROFILE\Documents\go-zero-trust-shop\services\auth-svc\migrate\migrations\0001_create_customers.down.sql"