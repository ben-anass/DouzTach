-- name: CreateWorkspace :one
INSERT INTO workspaces (
    owner_user_id,
    name,
    nickname,
    description,
    kind,
    theme,
    accent_color
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING
    id,
    owner_user_id,
    parent_workspace_id,
    name,
    nickname,
    description,
    kind,
    theme,
    accent_color,
    created_at,
    updated_at,
    deleted_at;

-- name: GetWorkspaceByID :one
SELECT
    id,
    owner_user_id,
    parent_workspace_id,
    name,
    nickname,
    description,
    kind,
    theme,
    accent_color,
    created_at,
    updated_at,
    deleted_at
FROM workspaces
WHERE id = $1
LIMIT 1;

-- name: ListWorkspacesForUser :many
SELECT
    w.id,
    w.owner_user_id,
    w.parent_workspace_id,
    w.name,
    w.nickname,
    w.description,
    w.kind,
    w.theme,
    w.accent_color,
    w.created_at,
    w.updated_at,
    w.deleted_at
FROM workspaces w
JOIN workspace_members wm ON wm.workspace_id = w.id
WHERE wm.user_id = $1
ORDER BY w.created_at DESC;

-- name: AddWorkspaceMember :one
INSERT INTO workspace_members (
    workspace_id,
    user_id,
    role
)
VALUES (
    $1, $2, $3
)
RETURNING
    id,
    workspace_id,
    user_id,
    role,
    joined_at;

-- name: ListWorkspaceMembers :many
SELECT
    id,
    workspace_id,
    user_id,
    role,
    joined_at
FROM workspace_members
WHERE workspace_id = $1
ORDER BY joined_at ASC;

-- name: IsWorkspaceMember :one
SELECT EXISTS (
    SELECT 1
    FROM workspace_members
    WHERE workspace_id = sqlc.arg(workspace_id)
      AND user_id = sqlc.arg(user_id)
);
