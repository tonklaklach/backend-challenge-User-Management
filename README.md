# Backend Golang Coding Test

Assignment source: [7-solutions/backend-challenge](https://github.com/7-solutions/backend-challenge).

## User Management API implementation

The implementation for part 1 is available in this repository. The original assignment is preserved below. Part 2 (lottery system design) is not included in this change.

### Structure

```text
main.go                 # dependency wiring, route registration, server lifecycle
internal/
  domain/               # user entity, repository contract, domain errors
  model/                # HTTP request and response models
  handler/              # decode requests, invoke services, map responses/errors
  service/              # validation, password hashing, authentication, ownership
  repository/           # MongoDB documents, indexes, queries, persistence error mapping
  db/                   # MongoDB connection
  auth/                 # HS256 JWT signing/verification and authenticated context
  middleware/           # authentication, request timeout, HTTP logging
  config/               # environment configuration
  worker/               # background user count every 10 seconds
```

The HTTP layer uses Echo v5. The domain does not depend on Echo, HTTP, BSON, or database drivers. Services depend on the domain repository interface; the MongoDB adapter implements that interface. HTTP and database models are separate from the domain entity. All routes and middleware are registered explicitly in `main.go`.

### Run

Requirements: Go 1.26.4 or later and a running MongoDB instance.

```sh
cp .env.example .env
# Edit .env and set JWT_SECRET to a random secret of at least 32 bytes.
# One way to generate it: openssl rand -hex 32
set -a
. ./.env
set +a
go mod download
go run .
```

The application does not load `.env` automatically. The commands above export it to the process environment. MongoDB defaults to `mongodb://localhost:27017`, database `backend_challenge`, and HTTP to `:8080`. Set `MONGO_URI`, `MONGO_DATABASE`, and `HTTP_ADDRESS` as needed. `JWT_TTL` defaults to `1h`. Startup fails if the JWT configuration, database connection, or unique email index is invalid.

See [API usage and examples](docs/api.md) for registration, login, JWT usage, and every user operation.

### Try with Postman

Import [the Postman collection](postman/backend-challenge.postman_collection.json). Set the collection variable `base_url` if the API is not running at `http://localhost:8080`; no environment file is required. Run requests in numerical order, or run the whole collection using Collection Runner.

Registration generates unique demo emails and saves `user_id`; login saves `access_token` automatically. The collection covers all endpoints plus expected 401/403 cases. The final cleanup folder logs into the secondary demo account and deletes both accounts created by the run. To inspect the users before deletion, run only the first two folders. Use a local/test database.

### Verification

```sh
go test -race . ./internal/service ./internal/auth ./internal/repository ./internal/config ./internal/handler ./internal/middleware ./internal/worker
go vet . ./internal/service ./internal/auth ./internal/repository ./internal/config ./internal/handler ./internal/middleware ./internal/db ./internal/worker
```

Tests use the standard `testing` package, repository/service fakes, and `httptest`; a live MongoDB instance is not required for unit tests. These tests do not substitute for a live MongoDB integration test.

MongoDB repository integration tests are opt-in. Start a separate test instance and run:

```sh
docker run -d --rm --name backend-challenge-test-mongo \
  -p 127.0.0.1:27018:27017 mongo:8.0

MONGO_TEST_URI=mongodb://127.0.0.1:27018 \
  go test -race -tags=integration ./internal/repository -count=1 -v

docker stop backend-challenge-test-mongo
```

The tagged tests require `MONGO_TEST_URI`. Each test creates and cleans up its own uniquely named database prefixed with `backend_challenge_test_`; it does not drop an application database. They verify CRUD round trips, field preservation, not-found results, canceled contexts, and unique-email enforcement under concurrent creates/updates. Worker tests inject tick events internally to verify error recovery and cancellation without waiting for the 10-second production interval.

### Assumptions and decisions

- Registration and login are public. Every `/users` endpoint requires a valid bearer token.
- Updates and deletion are restricted to the authenticated user's own ID. This rule is enforced in the service. There is no admin role or endpoint.
- Authenticated users may read user profiles/list users and create a new account. This retains the requested CRUD scope; only ID, name, email, and creation time are returned.
- Business inputs, including target user IDs, are accepted through JSON bodies, never custom headers or query parameters. JWT uses the standard `Authorization` header. Fetching one user uses `POST /users/get` to avoid GET bodies. Listing uses `GET /users` with no input.
- `POST /users` and `POST /auth/register` share the same creation service. Creating an account does not log into or assume that account.
- IDs are MongoDB ObjectIDs. Email is trimmed and lowercased for this application's uniqueness policy, enforced by a MongoDB unique index on both creation and update.
- Names contain 1–100 Unicode characters after trimming. Passwords contain 8–72 bytes, are not trimmed, and are stored as bcrypt hashes using the default cost. Responses never contain password hashes.
- PATCH changes only supplied name/email fields. An empty update is rejected. JSON `null` on optional update fields is treated as omitted. Unknown JSON fields and multiple JSON values are rejected; bodies are limited to 1 MiB.
- JWTs require HS256, issuer, subject, and expiry. They are stateless: deleting an account does not revoke an already-issued token before expiry. No refresh tokens or logout/revocation system are included. Use TLS outside local development.
- Listing returns all users, sorted by ID, as requested. Pagination would be necessary before exposing this operation for large production datasets.
- Requests propagate context with a 5-second deadline to MongoDB. The worker runs serially every 10 seconds with its own query deadline. SIGINT/SIGTERM stop the worker, drain HTTP requests, and disconnect MongoDB.
- gRPC and containerization bonuses are not implemented. The service/repository separation, validation, and graceful shutdown are included.

---

## Overview

This coding test has two parts.

| Section | Focus | Submission Type |
| --- | --- | --- |
| User Management API | Build a Golang user management API with MongoDB and JWT authentication | Code implementation |
| Lottery Search System | Design a real-world lottery ticket search solution with wildcard matching | Design proposal only (no code) |

## User Management API

### Objective (User Management API)

Build a RESTful API in Golang to manage users, using MongoDB for persistence, JWT for authentication, and clean code practices.

### Requirements (User Management API)

#### 1. User Model

Define a user entity with the following fields:

- `ID` (auto-generated)
- `Name` (string)
- `Email` (string, unique)
- `Password` (hashed)
- `CreatedAt` (timestamp)

#### 2. Authentication

Implement:

- User registration
- User authentication that returns a JWT token

JWT requirements:

- Protect endpoints with JWT
- Validate tokens via middleware
- Sign tokens using HMAC (`HS256`) with a secret key

#### 3. User Operations

Implement the following operations:

- Create a new user
- Fetch a user by ID
- List all users
- Update a user's name or email
- Delete a user

#### 4. MongoDB Integration

- Use the official Go MongoDB driver
- Persist and retrieve user data from MongoDB

#### 5. Middleware

- Implement logging middleware to capture HTTP method, path, and execution time

#### 6. Concurrency Task

- Run a background goroutine every 10 seconds to log the total number of users in the database

#### 7. Testing

- Write unit tests using Go's standard `testing` package
- Mock MongoDB interactions where appropriate

### Bonus (Optional, User Management API)

- **Containerization**: Add Docker and `docker-compose` support for the API and MongoDB
- **Abstraction**: Use Go interfaces to abstract MongoDB operations for better testability
- **Validation**: Implement input validation (for example, required fields and email format)
- **Graceful Shutdown**: Handle system signals using `context.Context`
- **gRPC Support**:
  - Define a `.proto` file for `CreateUser` and `GetUser`
  - Implement a gRPC server (optionally secure with token metadata)
- **Hexagonal Architecture**:
  - Structure the project using ports and adapters
  - Separate domain, application, and infrastructure layers
  - Decouple business logic from frameworks and drivers

### Deliverables (User Management API)

Provide a Git repository containing:

- `README.md` with setup and execution instructions
- A guide explaining how to generate and use JWT tokens
- Sample API requests and responses
- Documentation of assumptions or design decisions

### Evaluation Criteria (User Management API)

- Code quality, structure, and readability
- Correctness and completeness of the REST API
- Security and implementation of JWT
- Proper usage and abstraction of MongoDB
- Test coverage and effective mocking
- Idiomatic Go usage
- Bonus implementations (gRPC, Docker, validation, architecture)

## Lottery Search System

### Objective (Lottery Search System)

Design a real-world solution to search a large dataset of lottery tickets using pattern matching with wildcard support.

> This section is a design exercise. Do not implement code.

### Requirements (Lottery Search System)

#### 1. Data Volume

- Handle a dataset of **10 million** lottery tickets
- Each ticket is a 6-digit number

#### 2. Search Pattern

- Support a 6-character search pattern containing digits and wildcards (`*`)
- Example patterns:

| Pattern | Matches |
| --- | --- |
| `****23` | Numbers ending in `23` |
| `1****5` | Numbers starting with `1` and ending with `5` |
| `123***` | Numbers starting with `123` |

#### 3. Result Distribution

- Constraint: the same search pattern should not return the same ticket to multiple users at the same time
- Propose a distribution mechanism so matching tickets are assigned without duplicate simultaneous selection

#### 4. Performance

- Ensure the search is performant for `10M+` records
- Propose an efficient approach for querying and allocation

#### 5. Real-World Design Proposal (No Code Required)

- Recommend the database/storage technology you would use in production and explain why
- Describe the algorithm and indexing strategy used for wildcard pattern matching
- Explain how you would prevent duplicate simultaneous results for the same pattern (for example, locking, reservation, or atomic allocation)
- No code implementation is required; provide a solution/design only

### Deliverables (Lottery Search System)

Submit a design document only (no code implementation) that includes:

- Proposed solution architecture, data structures, and algorithms
- Recommended production database/storage choice with justification (for example, query performance, concurrency handling, operational simplicity)
- Performance analysis summarizing efficiency and tradeoffs
- Concurrency/distribution strategy explaining how duplicate results are avoided for the same pattern

### Evaluation Criteria (Lottery Search System)

- Feasibility: the solution addresses the stated requirements
- Performance: the search approach is efficient for the target scale
- Correctness: the distribution constraint is handled correctly
- Real-world practicality: the database/storage and concurrency approach are appropriate for production use
- Creativity: thoughtful use of data structures and algorithms
