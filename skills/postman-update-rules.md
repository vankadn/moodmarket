# Postman Collection Update Rules

Any time a new endpoint is added, an existing endpoint's request/response shape changes,
or an endpoint is removed — update the Postman collection as part of the same session.
Never leave the collection stale after a backend change.

## Files to update

| File | When |
|------|------|
| `postman/InvestIQ.postman_collection.json` | Always — new/changed/removed requests |
| `backend/internal/api/handlers/openapi.yaml` | Always — spec must match the implementation |
| `backend/internal/api/router/routes.go` | When adding a new URI constant |
| `backend/internal/api/router/router.go` | When registering a new route |

## Checklist for a new endpoint

1. Add URI constant to `router/routes.go`
2. Register handler in `router/router.go` using the constant
3. Add path + schemas to `openapi.yaml`
4. Add a request to the correct folder in `InvestIQ.postman_collection.json`
   - Use `{{baseUrl}}` and `{{authToken}}` — never hardcode URLs or tokens
   - Add path-param variables (e.g. `{{orderId}}`, `{{docId}}`) as collection variables
   - If the endpoint requires a specific setup order, add a `description` field explaining it

## Postman request format rules

- `auth`: inherit from collection (bearer `{{authToken}}`) unless the endpoint is public — then set `"auth": { "type": "noauth" }`
- `body.mode`: `"raw"` + `"language": "json"` for JSON; `"formdata"` for multipart
- Keep example bodies realistic — use the same field names as the actual Go struct JSON tags
- If a request returns an ID used by another request (e.g. invest → order_id), add a test script that auto-sets the variable:
  ```json
  { "listen": "test", "script": { "exec": ["var j = pm.response.json(); if (j.order_id) pm.collectionVariables.set('orderId', j.order_id);"] } }
  ```

## What NOT to do

- Do not hardcode `localhost:8080` or the Railway URL — always `{{baseUrl}}`
- Do not add secrets (API keys, tokens) to the collection file — those go in environment files only
- Do not commit `postman/*.postman_environment.json` if it contains real credentials
