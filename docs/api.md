# User Management API

Base URL: `http://localhost:8080`. Requests with a body require `Content-Type: application/json`. All business inputs are JSON body fields; the only authentication input is `Authorization: Bearer <token>`.

| Method | Route | Body | Authentication | Success |
| --- | --- | --- | --- | --- |
| POST | `/auth/register` | name, email, password | Public | 201 |
| POST | `/auth/login` | email, password | Public | 200 |
| POST | `/users` | name, email, password | JWT | 201 |
| POST | `/users/get` | id | JWT | 200 |
| GET | `/users` | None | JWT | 200 |
| PATCH | `/users` | id, name and/or email | JWT, owner only | 200 |
| DELETE | `/users` | id | JWT, owner only | 204 |

## Register

```sh
curl -i http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","password":"password123"}'
```

Example response (`201 Created`):

```json
{"id":"507f1f77bcf86cd799439011","name":"Alice","email":"alice@example.com","created_at":"2026-11-02T08:00:00Z"}
```

Copy the actual generated `id` for subsequent requests; the ID and timestamp above are illustrative.

## Login and JWT

```sh
curl -i http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password123"}'
```

Example response (`200 OK`):

```json
{"access_token":"<signed JWT>","token_type":"Bearer","expires_in":3600}
```

Use the returned JWT, not the signing secret, in the Authorization header. Log in again when it expires.

```sh
TOKEN='<access_token from login>'
USER_ID='<id from registration>'
```

## Create another user

```sh
curl -i http://localhost:8080/users \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Bob","email":"bob@example.com","password":"password456"}'
```

Returns `201 Created` with the same user response shape as registration. The current token still belongs to Alice.

## Get a user

```sh
curl -i http://localhost:8080/users/get \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"id\":\"$USER_ID\"}"
```

Returns `200 OK` with the user response shape above.

## List users

```sh
curl -i http://localhost:8080/users -H "Authorization: Bearer $TOKEN"
```

Returns `200 OK` with an array of user responses, or `[]` when there are no users.

## Update your own user

```sh
curl -i -X PATCH http://localhost:8080/users \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"id\":\"$USER_ID\",\"name\":\"Alice Updated\",\"email\":\"alice.new@example.com\"}"
```

Returns `200 OK` with the updated user. Either name or email may be omitted. Changes to another user's account return `403 Forbidden`.

## Delete your own user

```sh
curl -i -X DELETE http://localhost:8080/users \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"id\":\"$USER_ID\"}"
```

Returns `204 No Content` with an empty body. Repeating the deletion returns `404` while the token remains valid.

## Errors

Errors returned by application handlers and authentication middleware use this shape:

```json
{"error":"email already exists"}
```

| Status | Meaning |
| --- | --- |
| 400 | Invalid JSON, user ID, or business input |
| 401 | Missing/invalid/expired JWT, or invalid login credentials |
| 403 | Attempt to update/delete another user's account |
| 404 | User does not exist |
| 409 | Email already exists |
| 413 | Request body exceeds 1 MiB |
| 415 | Missing or unsupported Content-Type |
| 500 | Unexpected server/database error; internal details are not returned |

Echo handles routing. Unknown routes and unsupported HTTP methods use the same JSON error shape through the centralized error handler (404/405); authentication still applies to the protected `/users` group. Logs contain method, path, duration, and server-side errors; request bodies, passwords, and JWTs are not logged.
