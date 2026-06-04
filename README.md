# FicMart Payment Gateway

A production-grade payment gateway built in Go for FicMart, a fictional
e-commerce platform. The gateway sits between FicMart's order service and
a mock bank API, handling the full payment lifecycle with strict state
management, idempotency, and resilient failure handling.

## What's in this repo

```
payment-gateway/
├── bank/          → Mock bank API
├── docker/        → Docker Compose for running everything together
└── gateway/       → Payment gateway
```

## Architecture

```
FicMart → Gateway API → Service Layer → Bank Client → Mock Bank
↓
PostgreSQL (Neon)
```

The gateway is organized into distinct layers:

- **domain** — core payment types and state machine rules, zero external dependencies
- **repository** — all database queries
- **bank** — HTTP client that talks to the mock bank with retry logic
- **service** — orchestrates operations: validate state, call bank, save result
- **handlers** — HTTP layer, parses requests and writes responses
- **middleware** — idempotency checking, request logging, panic recovery

## Payment Lifecycle

```
PENDING → AUTHORIZED → CAPTURED → REFUNDED
↓
VOIDED
```

Invalid transitions are rejected at the domain layer regardless of what
the bank accepts. A voided payment cannot be captured. A captured payment
cannot be voided.

## API Endpoints

| Method | Endpoint                    | Description                         |
| ------ | --------------------------- | ----------------------------------- |
| `POST` | `/payments/authorize`       | Reserve funds on a card             |
| `POST` | `/payments/{id}/capture`    | Charge previously authorized funds  |
| `POST` | `/payments/{id}/void`       | Cancel authorization before capture |
| `POST` | `/payments/{id}/refund`     | Return money after capture          |
| `GET`  | `/payments?order_id=xxx`    | Get payment status by order         |
| `GET`  | `/payments?customer_id=xxx` | Get all payments for a customer     |
| `GET`  | `/health`                   | Health check                        |

All write endpoints require an `Idempotency-Key` header.

### Example: Authorize

```bash
curl -X POST http://localhost:8088/payments/authorize \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: unique-key-001" \
  -d '{
    "order_id": "order-123",
    "customer_id": "customer-456",
    "amount": 5000,
    "currency": "USD",
    "card_number": "4111111111111111",
    "card_expiry": "12/2030",
    "card_cvv": "123"
  }'
```

Response:

```json
{
  "payment_id": "4262730c-3a1a-4489-b7a1-b761b7a7eae3",
  "order_id": "order-123",
  "customer_id": "customer-456",
  "amount": 5000,
  "currency": "USD",
  "state": "AUTHORIZED",
  "bank_auth_id": "auth_b424a765-...",
  "created_at": "2026-06-01T17:57:06Z",
  "authorized_at": "2026-06-01T17:57:06Z"
}
```

## Key Features

**Idempotency** — duplicate requests with the same key return the original
response without hitting the bank again. Replayed responses include an
`X-Idempotent-Replayed: true` header.

**Retry logic** — transient failures (5xx, timeouts, network errors) are
retried with exponential backoff and jitter. Permanent failures (expired
card, insufficient funds) are never retried.

**Crash safety** — the gateway writes a `PENDING` record before calling
the bank. If it crashes mid-operation, the orphaned record is detectable
and recoverable.

## Test Cards

| Card Number      | CVV | Expiry  | Balance | Use Case           |
| ---------------- | --- | ------- | ------- | ------------------ |
| 4111111111111111 | 123 | 12/2030 | $10,000 | Happy path         |
| 4242424242424242 | 456 | 06/2030 | $500    | Limited balance    |
| 5555555555554444 | 789 | 09/2030 | $0      | Insufficient funds |
| 5105105105105100 | 321 | 03/2020 | $5,000  | Expired card       |

Amounts are in cents — `5000` = $50.00.

## Running Locally

**Prerequisites:** Docker, Docker Compose, Go 1.25+

**1. Clone the repo**

```bash
git clone https://github.com/Oliveszn/Payment-Gateway.git
cd Payment-Gateway
```

**2. Set up environment**

```bash
cp gateway/.env.example gateway/.env
# Edit gateway/.env and add your DATABASE_URL
```

**3. Start everything**

```bash
cd docker
docker compose up -d
```

**4. Run migrations**

```bash
cd gateway
make migrate-up
```

**5. Start the gateway**

```bash
make run
```

The gateway runs on `http://localhost:8088`.
The mock bank Swagger docs are at `http://localhost:8787/docs`.

## Available Commands

```bash
make run            # Start the gateway locally
make test           # Run all tests
make test-short     # Run unit tests only (no database needed)
make test-cover     # Run tests with coverage report
make migrate-up     # Run database migrations
make migrate-down   # Roll back last migration
make docker-up      # Start all containers
make docker-down    # Stop all containers
make docker-logs    # Tail gateway logs
make smoke-authorize # Quick smoke test against running gateway
make health         # Check gateway health
```

## Running Tests

```bash
cd gateway
make test
```

ok payment-gateway/internal/domain (13 tests)
ok payment-gateway/internal/service (9 tests)

- **Domain tests** — verify all state machine transitions and rejections
- **Service tests** — verify business logic with mocked bank and repository

## Stack

- **Language:** Go 1.25
- **Database:** PostgreSQL (Neon)
- **Migrations:** golang-migrate
- **Containerization:** Docker + Docker Compose
- **Testing:** testify

## Design Decisions

See [gateway/TRADEOFFS.md](gateway/TRADEOFFS.md) for a full breakdown of
architectural decisions, state management approach, retry strategy,
idempotency implementation, and what would change in production.
