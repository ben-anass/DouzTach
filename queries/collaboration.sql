-- name: CreateChatChannel :one
INSERT INTO chat_channels (
    workspace_id,
    name,
    kind
)
VALUES (
    $1, $2, $3
)
RETURNING
    id,
    workspace_id,
    name,
    kind,
    created_at,
    updated_at;

-- name: GetChatChannel :one
SELECT
    id,
    workspace_id,
    name,
    kind,
    created_at,
    updated_at
FROM chat_channels
WHERE id = $1
  AND workspace_id = $2;

-- name: ListChatChannels :many
SELECT
    id,
    workspace_id,
    name,
    kind,
    created_at,
    updated_at
FROM chat_channels
WHERE workspace_id = $1
ORDER BY name ASC;

-- name: CreateChatMessage :one
INSERT INTO chat_messages (
    workspace_id,
    channel_id,
    sender_id,
    content
)
VALUES (
    $1, $2, $3, $4
)
RETURNING
    id,
    workspace_id,
    channel_id,
    sender_id,
    content,
    created_at,
    updated_at,
    deleted_at;

-- name: ListChannelMessages :many
SELECT
    id,
    workspace_id,
    channel_id,
    sender_id,
    content,
    created_at,
    updated_at,
    deleted_at
FROM chat_messages
WHERE channel_id = sqlc.arg(channel_id)
  AND workspace_id = sqlc.arg(workspace_id)
  AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size)
OFFSET sqlc.arg(page_offset);

-- name: CreateNotification :one
INSERT INTO notifications (
    recipient_user_id,
    workspace_id,
    event_type,
    related_entity_id,
    message
)
VALUES (
    $1, $2, $3, $4, $5
)
RETURNING
    id,
    recipient_user_id,
    workspace_id,
    event_type,
    related_entity_id,
    message,
    created_at,
    read_at;

-- name: CreateActivityLog :one
INSERT INTO activity_logs (
    actor_user_id,
    workspace_id,
    action_type,
    entity_type,
    entity_id,
    previous_value,
    new_value
)
VALUES (
    sqlc.arg(actor_user_id),
    sqlc.arg(workspace_id),
    sqlc.arg(action_type),
    sqlc.arg(entity_type),
    sqlc.arg(entity_id),
    sqlc.arg(previous_value)::jsonb,
    sqlc.arg(new_value)::jsonb
)
RETURNING
    id,
    actor_user_id,
    workspace_id,
    action_type,
    entity_type,
    entity_id,
    previous_value,
    new_value,
    created_at;

-- name: ListNotificationsForUser :many
SELECT
    id,
    recipient_user_id,
    workspace_id,
    event_type,
    related_entity_id,
    message,
    created_at,
    read_at
FROM notifications
WHERE recipient_user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size)
OFFSET sqlc.arg(page_offset);

-- name: MarkNotificationRead :one
UPDATE notifications
SET read_at = COALESCE(read_at, NOW())
WHERE id = $1
  AND recipient_user_id = $2
RETURNING
    id,
    recipient_user_id,
    workspace_id,
    event_type,
    related_entity_id,
    message,
    created_at,
    read_at;

-- name: ListActivityForWorkspace :many
SELECT
    id,
    actor_user_id,
    workspace_id,
    action_type,
    entity_type,
    entity_id,
    previous_value,
    new_value,
    created_at
FROM activity_logs
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size)
OFFSET sqlc.arg(page_offset);
