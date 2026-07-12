# Sahata Worship API

Backend Go berarsitektur `handler -> usecase -> repository` dengan MySQL.

## Setup

1. Buat DB: `CREATE DATABASE sahata_worship CHARACTER SET utf8mb4;`
2. Salin `.env.example` ke `.env` dan export nilainya ke terminal.
3. Migration: `migrate -path migrations -database "mysql://root:password@tcp(localhost:3306)/sahata_worship" up`
4. Jalankan: `go run ./cmd/api`

Health check: `GET http://localhost:8080/health`.

## API

Publik: `POST /api/v1/auth/register` dan `POST /api/v1/auth/login`.

Endpoint di bawah memakai `Authorization: Bearer <token>`:

- `GET/POST /api/v1/rooms`; `GET/PUT/DELETE /api/v1/rooms/{id}`
- `GET /api/v1/members?roomId={id}`; `POST /api/v1/members`; `PUT/DELETE /api/v1/members/{id}`
- `GET/POST /api/v1/cues`; `PUT/DELETE /api/v1/cues/{id}`
- `GET /api/v1/activities?roomId={id}`; `POST /api/v1/activities`
- `GET/PUT /api/v1/settings`

Contoh register:

```json
{"name":"Daniel Siregar","email":"daniel@sahata.church","password":"password123","role":"Music Director"}
```
