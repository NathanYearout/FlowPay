# FlowPay

This is an example project I used to learn **Go**, **Kafka**, **SQLC**, and **Gin**. It's a simple double-entry ledger system for tracking transactions.

## Setup

You'll need Docker and Go installed. To get the database and Kafka running:

```bash
make run
```

## Testing the API

Here are some `curl` snippets to test out the endpoints.

### 1. Health Check

```bash
curl http://localhost:8080/health
```

### 2. Create Accounts

Create a couple of accounts to move money between.

**Account A:**

```bash
curl -X POST http://localhost:8080/accounts
     -H "Content-Type: application/json"
     -d '{
       "owner": "user_a",
       "asset_type": "USD",
       "account_type": "asset"
     }'
```

**Account B:**

```bash
curl -X POST http://localhost:8080/accounts
     -H "Content-Type: application/json"
     -d '{
       "owner": "user_b",
       "asset_type": "USD",
       "account_type": "liability"
     }'
```

### 3. Create a Transaction

Use the account IDs returned from the calls above. Remember that ledger entries must sum to zero and include an `Idempotency-Key`.

```bash
curl -X POST http://localhost:8080/transactions
     -H "Content-Type: application/json"
     -H "Idempotency-Key: txn_123"
     -d '{
       "type": "payment",
       "entries": [
         {
           "account_id": "PASTE_ID_A_HERE",
           "amount": "-50.00"
         },
         {
           "account_id": "PASTE_ID_B_HERE",
           "amount": "50.00"
         }
       ]
     }'
```

### 4. Check Balance

Check the balance of an account to verify the transaction.

```bash
curl http://localhost:8080/accounts/<ID>/balance
```
