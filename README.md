# rooms

Room management microservice for the GAZ platform.

## Overview

Manages activity rooms where users meet up — creation, discovery, joining, and membership. Enforces per-subscription limits on room count and participant capacity. Notifies the `messenger` service over its internal HTTP API whenever room membership changes so the chat stays in sync.

## Architecture

Single REST server backed by PostgreSQL. Integrates with two other microservices via gRPC:

- **auth_users / Token gRPC** — validates JWTs on every authenticated request
- **auth_users / User gRPC** — fetches email-verified status and subscription tier before a room is created

The `messenger` integration is optional: if `MESSENGER_INTERNAL_URL` is not set the service starts in standalone mode with a no-op notifier.

## Tech Stack

- **Go 1.25** — standard library `log/slog` for structured logging
- **Gin** — HTTP routing with `gin.Recovery()` middleware
- **PostgreSQL** + **pgx/v5** — connection pool, migrations via `golang-migrate`
- **gRPC** — consumes `auth_users` token and user services
- **HTTP client** — calls `messenger` internal API to sync chat membership

## Project Structure

```
rooms/
├── cmd/
│   ├── app/                    # entrypoint
│   └── migrator/               # standalone DB migrator
├── internal/
│   ├── app/                    # wires server, middleware, and clients
│   ├── config/                 # env-based config with validation
│   ├── core/                   # domain types, errors, subscription limits
│   ├── infrastructure/
│   │   ├── grpcclient/         # gRPC client for auth_users user service
│   │   └── messengerclient/    # HTTP client for messenger internal API
│   ├── middleware/             # JWT auth middleware (via gRPC)
│   ├── repository/             # PostgreSQL queries
│   ├── service/                # business logic and subscription enforcement
│   └── transport/rest/         # HTTP handlers
└── migrations/                 # SQL up/down migrations
```

## API

### Public endpoints (no auth)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/room-types` | List available room types |
| `GET` | `/cities` | List cities that have at least one room |
| `GET` | `/rooms` | List rooms with optional filters and cursor pagination |
| `GET` | `/rooms/:id` | Get room details |
| `GET` | `/rooms/:id/participants` | List room participants |

### Protected endpoints (Bearer JWT required)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/rooms` | Create a room |
| `PUT` | `/rooms/:id` | Update room (owner only) |
| `DELETE` | `/rooms/:id` | Delete room (owner only) |
| `POST` | `/rooms/:id/join` | Join a room |
| `POST` | `/rooms/:id/leave` | Leave a room |
| `GET` | `/users/me/rooms` | Get rooms the current user belongs to |

### Listing and pagination

`GET /rooms` supports the following query parameters:

| Parameter | Description |
|-----------|-------------|
| `city` | Filter by city |
| `type` | Filter by room type name |
| `limit` | Page size (default `20`, max `100`) |
| `last_id` | Cursor — ID of the last item on the previous page |

The response includes a `next_cursor` field. Pass it as `?last_id=` to fetch the next page.

## Subscription System

Every user has a subscription tier that controls what they can do:

| Tier | Rooms they can create | Max participants per room |
|------|-----------------------|--------------------------|
| `default` | 3 | 10 |
| `vip` | 5 | 20 |
| `super` | 10 | 50 |

Tier is fetched from `auth_users` via gRPC on every `CreateRoom` and `UpdateRoom` call. Attempts to exceed the limits return `409 Conflict`.

## Room Types

Seeded at migration time:

`бары` · `клубы` · `настолки` · `прогулка` · `посидеть` · `спорт` · `работа` · `квесты`

The full list is always available at `GET /room-types`.

## Messenger Integration

When membership changes, `rooms` calls the `messenger` internal API:

| Event | Messenger endpoint |
|-------|--------------------|
| Room created | `POST /internal/chats` |
| Room deleted | `DELETE /internal/chats/:room_id` |
| User joined | `POST /internal/chats/:room_id/members` |
| User left | `DELETE /internal/chats/:room_id/members/:user_id` |

All calls are fire-and-forget with a warning log on failure so a messenger outage never affects room operations.

## Configuration

Copy `internal/config/config.env.example` to `internal/config/config.env`. In production supply environment variables directly.

| Variable | Description |
|----------|-------------|
| `REST_ROOMS_HOST_ADDRESS` | Listen address (e.g. `:8083`) |
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` | PostgreSQL connection |
| `AUTH_GRPC_ADDR` | auth_users token gRPC address (e.g. `localhost:9091`) |
| `USER_GRPC_ADDR` | auth_users user gRPC address (e.g. `localhost:9092`) |
| `MESSENGER_INTERNAL_URL` | messenger base URL (optional, e.g. `http://localhost:8080`) |
| `MESSENGER_INTERNAL_KEY` | Shared secret for messenger internal API (`X-Internal-Key`) |
| `SERV_TIMEOUT` | Graceful shutdown timeout (default `5s`) |
| `LOG_LEVEL` | `local` / `dev` / `prod` |

## Database Migrations

```bash
go run ./cmd/migrator
```

Migrations live in `migrations/` and run in order. Each migration has an `up` and `down` file.

## Running Locally

```bash
# 1. Start dependency
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=secret postgres:16

# 2. Make sure auth_users is running (rooms needs its gRPC endpoints)

# 3. Configure
cp internal/config/config.env.example internal/config/config.env
# edit config.env

# 4. Migrate
go run ./cmd/migrator

# 5. Run
go run ./cmd/app
```

## Build

```bash
go build -o rooms ./cmd/app
go build -o migrator ./cmd/migrator
```
