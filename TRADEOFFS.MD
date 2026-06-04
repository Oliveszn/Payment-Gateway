# Payment Gateway Design & Tradeoffs

## 1. Architecture: Why structure it this way?

I implemented this gateway using some Domain-Driven Design (DDD) principles, organizing the code into distinct layers: domain, repository, bank, service, handlers, middleware

- **Dependency isolation:** The domain layer is the core of the system and has
  zero external dependencies. Business rules like state transitions live here and
  are never coupled to database schemas or external APIs. This means the rules
  stay stable even if the bank API or database changes.

- **Service layer separation:** Rather than putting business logic directly in
  HTTP handlers, the service layer owns orchestration — authorize, capture, void,
  refund, and queries. Handlers only parse requests and write responses. This
  separation means the same business logic could be exposed over gRPC or a queue
  consumer without rewriting anything.

- **Testability via interfaces:** By defining a `BankClient` interface in the
  service layer, the real bank client and a mock are interchangeable. This lets
  unit tests run without a live bank or database, making them fast and
  deterministic.

## 2. State Management: How and why?

Payment state is stored in PostgreSQL and transitions follow a strict one-way
machine: `PENDING → AUTHORIZED → CAPTURED → REFUNDED` with `AUTHORIZED → VOIDED`
as the only branch.

The gateway writes a `PENDING` record to the database _before_ calling the bank.
This is intentional — if the gateway crashes after the bank authorizes but before
the result is saved, the orphaned `PENDING` record is detectable on recovery. A
reconciliation worker can query the bank using the stored idempotency key to
determine the real outcome and update the record accordingly.

State transitions are enforced at the domain layer regardless of what the bank
accepts. A voided payment cannot be captured. A captured payment cannot be voided.
The gateway is the source of truth, not the bank.

## 3. Failure Handling: Retries and Partial Failures

Errors are split into two categories:

- **Transient:** Temporary failures worth retrying — 5xx responses, network
  timeouts, connection errors. The bank will likely succeed on a subsequent attempt.
- **Permanent:** Failures that will never succeed regardless of retries — expired
  card, insufficient funds, invalid CVV. Retrying wastes time and could confuse
  the caller.

Transient failures are retried using exponential backoff with jitter. Backoff
increases the delay between attempts exponentially (100ms → 200ms → 400ms),
reducing pressure on an already struggling service. Jitter adds randomness to each
delay to prevent the "thundering herd" problem — without it, multiple clients
retrying in lockstep can overwhelm a recovering service all at once. Together they
smooth retry traffic and improve recovery time during partial outages.

## 4. Idempotency: Implementation and Edge Cases

Idempotency is enforced at the database level using a dedicated `idempotency_keys`
table in PostgreSQL. Each entry stores the idempotency key, operation, payment ID,
and the exact response bytes that were returned.

When a duplicate request arrives with the same key, the gateway queries this table
first. If a record exists, the stored response is returned immediately with an
`X-Idempotent-Replayed: true` header — the bank is never called again.

Only successful responses (2xx) are stored. Failed responses like insufficient
funds or expired cards are intentionally not stored, so the caller can fix the
issue and retry with the same key to get a fresh attempt. Storing failures would
permanently poison the key, preventing legitimate retries.

## 5. What I'd Do Differently in Production

- **Card vaulting:** Raw card details should never sit in the payments table. In
  production I'd integrate a dedicated vault (e.g. HashiCorp Vault or a PCI-DSS
  compliant tokenization service) and store only a token reference.

- **Redis for idempotency lookups:** At scale, hitting PostgreSQL on every request
  to check idempotency adds latency. Moving the idempotency layer to Redis gives
  sub-millisecond lookups with PostgreSQL as a durable fallback.

- **Reconciliation worker:** A background worker that periodically scans for
  `PENDING` payments older than a threshold and reconciles them against the bank.
  This handles the crash-between-bank-call-and-save scenario automatically rather
  than relying on manual recovery.

- **Chaos/fault injection testing:** I'd add tests that simulate crashes at the
  exact point between the bank response and the database commit to verify the
  recovery path works correctly under real failure conditions.
