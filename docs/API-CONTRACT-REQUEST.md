# Backend API contract — what the CLI needs

**Status:** blocking the terminal client.
**Owner of the backend:** Member 2 (nani) — auth, users, workspaces, members, invitations, permissions.
**Owner of data modules:** Member 3 (med) — tasks, epics, scrum, chat, notifications, activity.

The terminal client (`client/`) is complete: commands, prompts, tables, dashboard,
and the HTTP transport are implemented. What is missing is the **contract
itself** — the endpoint paths, payload schemas, and response envelopes. The
client deliberately refuses to guess them (`api.ErrContractPending`), so every
data command currently fails with:

```
Workspaces is not connected yet: backend API contract is not available yet (waiting for nani)
```

Nothing below is implemented on the client side yet; this document is the
request. Once the paths and schemas are confirmed, the work is a single file
(`client/api/<service>.go`) per service plus a call to `newHTTPServices(...)`
instead of `newPendingServices(...)` in `client/api/services.go`.

## 1. Transport conventions the client already assumes

| Concern | What the client expects | Where |
|---|---|---|
| Base URL | configured by the user (`douztech setup`), no trailing slash | `config.Config.APIBaseURL` |
| Auth header | `Authorization: Bearer <access_token>` | `api.Client.Do` |
| Request body | `application/json` | `api.Client.Do` |
| Response body | JSON, max 8 MiB | `api.Client` |
| Accept | `application/json` | `api.Client.Do` |
| User-Agent | `douztech-cli` | `api.Client` |

### Error envelope

Any non-2xx response must be JSON. The client accepts either shape:

```json
{ "error": { "code": "INVALID_CREDENTIALS", "message": "Email or password is incorrect", "details": null } }
```

```json
{ "code": "VALIDATION_FAILED", "message": "email is required", "request_id": "..." }
```

`X-Request-ID` (or `Request-ID`) and `Retry-After` are also read from headers.

### Standard status codes the client maps

| Status | Client behaviour |
|---|---|
| 401 | treated as "not signed in" (exit code 3) |
| 403 | treated as "not permitted" (exit code 3) |
| 404 | treated as "not found" (exit code 5) |
| 409 | reported as a conflict |
| 429 | reported with `Retry-After` (exit code 6) |
| 5xx | reported as a server error (exit code 7) |

### Pagination

The client currently sends `limit` as a query parameter for activity and chat
messages and expects a plain JSON array. If the backend returns a wrapper
(`{"items": [...], "next_cursor": "..."}`), state the exact shape and it will be
adapted in one place.

## 2. Authentication (`AuthService`)

| Method | Path | Request | Response |
|---|---|---|---|
| POST | `/auth/login` | `{"email":"...","password":"..."}` | `{"access_token":"...","refresh_token":"...","token_type":"Bearer","expires_at":"RFC3339","user":{...}}` |
| POST | `/auth/register` | `{"name":"...","email":"...","password":"..."}` | same as login |
| POST | `/auth/logout` | — | `204` |
| GET | `/auth/me` (or `/users/me`) | — | `{"id","name","username","email","avatar_url"}` |

Client-side types: `api.LoginRequest`, `api.RegisterRequest`, `api.AuthResult`,
`api.User`.

**Note:** the password is never accepted as a command-line flag; it is read from
a masked prompt or stdin. The client stores the token in `session.json`
(0600) and never prints it.

## 3. Users (`AuthService.Profile`)

The user object shape used by the client:

```json
{ "id": "u_123", "name": "Jane Doe", "username": "jane", "email": "jane@example.com", "avatar_url": "" }
```

`id` must be present: `--mine` filtering and the dashboard depend on it.

## 4. Workspaces (`WorkspaceService`)

| Method | Path | Notes |
|---|---|---|
| GET | `/workspaces` | workspaces the caller belongs to |
| POST | `/workspaces` | `{"name":"...","description":"..."}` → the created workspace |
| GET | `/workspaces/{id}/members` | list of members |

Workspace object:

```json
{ "id": "w_1", "name": "Platform", "description": "", "role": "OWNER", "member_count": 4, "created_at": "RFC3339" }
```

Member object:

```json
{ "user": { "id": "u_1", "name": "Jane", "username": "jane", "email": "jane@example.com" }, "role": "MEMBER", "joined_at": "RFC3339" }
```

Needed by: `douztech workspace list|create|switch|members`, dashboard header,
workspace picker.

## 5. Tasks (`TaskService`)

| Method | Path | Notes |
|---|---|---|
| GET | `/workspaces/{ws}/tasks` | optional `status`, `priority`, `type`, `sprint`, `epic`, `assignee`, `q`, `backlog`, `mine` |
| GET | `/tasks/{id}` | single task |
| POST | `/workspaces/{ws}/tasks` | create |
| PATCH | `/tasks/{id}` | partial update; omitted = unchanged, `null` = clear |
| DELETE | `/tasks/{id}` | `204` |

Task object:

```json
{
  "id": "t_1", "key": "DT-12", "title": "...", "description": "...",
  "workspace_id": "w_1", "sprint_id": "", "epic_id": "",
  "type": "TASK", "status": "TODO", "priority": "MEDIUM",
  "assignee": { "id": "u_1", "name": "Jane", "username": "jane", "email": "jane@example.com" },
  "reporter": { ... }, "due_date": "RFC3339|null", "story_points": 3,
  "created_at": "RFC3339", "updated_at": "RFC3339"
}
```

Enums (already fixed in the client, `client/api/models.go`):

* `status`: `TODO`, `IN_PROGRESS`, `REVIEW`, `DONE`
* `type`: `TASK`, `BUG`, `STORY`, `EPIC`, `SUBTASK`
* `priority`: `LOW`, `MEDIUM`, `HIGH`, `URGENT`

Create payload: `api.CreateTaskRequest`; update payload: `api.UpdateTaskRequest`
(pointer fields, omitted = unchanged).

**Filtering note:** the client applies `api.TaskFilter.Matches` client-side as a
safety net, so returning a broader result set is harmless but wasteful.

## 6. Epics (`EpicService`)

| Method | Path |
|---|---|
| GET | `/workspaces/{ws}/epics` |
| GET | `/epics/{id}` |

```json
{ "id":"e_1","key":"EP-1","title":"...","description":"...","status":"OPEN","workspace_id":"w_1",
  "owner":{...},"due_date":null,"task_count":10,"done_count":4,"created_at":"...","updated_at":"..." }
```

`task_count` / `done_count` drive the progress bars; send them, or the client
shows 0%.

## 7. Sprints (`SprintService`)

| Method | Path |
|---|---|
| GET | `/workspaces/{ws}/sprints` |
| GET | `/sprints/{id}` |

```json
{ "id":"s_1","name":"Sprint 4","goal":"...","status":"ACTIVE","workspace_id":"w_1",
  "start_date":"...","end_date":"...","task_count":8,"done_count":3,"created_at":"...","updated_at":"..." }
```

`status`: `PLANNED`, `ACTIVE`, `COMPLETED`. The dashboard and the sprint board
both look for exactly one `ACTIVE` sprint.

## 8. Notifications (`NotificationService`)

| Method | Path |
|---|---|
| GET | `/notifications` |
| POST | `/notifications/{id}/read` |
| POST | `/notifications/read-all` |

```json
{ "id":"n_1","title":"Assigned to you","body":"DT-12","type":"TASK_ASSIGNED","read":false,"link":"","created_at":"..." }
```

## 9. Activity (`ActivityService`)

| Method | Path |
|---|---|
| GET | `/workspaces/{ws}/activity?limit=30` |

```json
{ "id":"a_1","actor_id":"u_1","actor_name":"Jane","action":"updated","subject":"DT-12","subject_id":"t_1","created_at":"..." }
```

`actor_name` is displayed directly, so send it rather than only the ID.

## 10. Chat (`ChatService`)

| Method | Path | Notes |
|---|---|---|
| GET | `/chat/channels` | channels visible to the caller |
| GET | `/chat/channels/{id}/messages?limit=50` | message history |
| POST | `/chat/channels/{id}/messages` | `{"body":"..."}` → stored message |

```json
{ "id":"c_1","name":"general","description":"","member_count":12 }
```

```json
{ "id":"m_1","channel_id":"c_1","sender_id":"u_1","sender_name":"Jane","body":"hello","created_at":"..." }
```

**Realtime:** `client/websocket/` is empty and unused. Message history is the
only chat surface implemented; live delivery needs a documented websocket
endpoint and frame format before it can be built. Do not send a
`101 Switching Protocols` contract later without the frame schema.

## 11. What the client will do when the contract lands

1. Add `client/api/http_services.go` implementing the eight interfaces.
2. Switch `api.NewServices` to use it and set `Status.ContractReady = true`.
3. Delete the `pending*` types in `client/api/services.go`.
4. Point `docs/` at the endpoint list above.
5. Commands, screens, prompts, and tables need **no** changes.

Until then, `douztech doctor` reports `contract: pending` and data commands exit
with code **4**. No command invents data.
