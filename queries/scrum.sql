-- name: CreateEpic :one
INSERT INTO epics (
    workspace_id,
    title,
    description,
    status,
    priority,
    color,
    start_date,
    end_date
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING
    id,
    workspace_id,
    title,
    description,
    status,
    priority,
    color,
    start_date,
    end_date,
    created_at,
    updated_at,
    deleted_at;

-- name: ListEpicsForWorkspace :many
SELECT
    id,
    workspace_id,
    title,
    description,
    status,
    priority,
    color,
    start_date,
    end_date,
    created_at,
    updated_at,
    deleted_at
FROM epics
WHERE workspace_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetEpic :one
SELECT
    id,
    workspace_id,
    title,
    description,
    status,
    priority,
    color,
    start_date,
    end_date,
    created_at,
    updated_at,
    deleted_at
FROM epics
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL
LIMIT 1;

-- name: UpdateEpic :one
UPDATE epics
SET
    title = $3,
    description = $4,
    status = $5,
    priority = $6,
    color = $7,
    start_date = $8,
    end_date = $9,
    updated_at = NOW()
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL
RETURNING
    id,
    workspace_id,
    title,
    description,
    status,
    priority,
    color,
    start_date,
    end_date,
    created_at,
    updated_at,
    deleted_at;

-- name: SoftDeleteEpic :one
UPDATE epics
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL
RETURNING id, workspace_id, title;

-- name: GetEpicProgress :one
SELECT
    COUNT(items.id)::BIGINT AS total_items,
    COUNT(items.id) FILTER (WHERE items.status = 'done')::BIGINT AS completed_items
FROM epics e
LEFT JOIN (
    SELECT us.id, us.status
    FROM user_stories us
    WHERE us.epic_id = $1
      AND us.workspace_id = $2
      AND us.deleted_at IS NULL
    UNION ALL
    SELECT wi.id, wi.status
    FROM work_items wi
    JOIN user_stories us
      ON us.id = wi.user_story_id
     AND us.workspace_id = wi.workspace_id
     AND us.deleted_at IS NULL
    WHERE us.epic_id = $1
      AND wi.workspace_id = $2
      AND wi.deleted_at IS NULL
) items ON TRUE
WHERE e.id = $1
  AND e.workspace_id = $2
  AND e.deleted_at IS NULL
GROUP BY e.id;

-- name: CreateUserStory :one
INSERT INTO user_stories (
    workspace_id,
    epic_id,
    title,
    description,
    acceptance_criteria,
    status,
    priority,
    assignee_id,
    reporter_id,
    sprint_id,
    story_points
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING
    id,
    workspace_id,
    epic_id,
    title,
    description,
    acceptance_criteria,
    status,
    priority,
    assignee_id,
    reporter_id,
    sprint_id,
    story_points,
    created_at,
    updated_at,
    deleted_at;

-- name: ListUserStoriesForWorkspace :many
SELECT
    id,
    workspace_id,
    epic_id,
    title,
    description,
    acceptance_criteria,
    status,
    priority,
    assignee_id,
    reporter_id,
    sprint_id,
    story_points,
    created_at,
    updated_at,
    deleted_at
FROM user_stories
WHERE workspace_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetUserStory :one
SELECT
    id,
    workspace_id,
    epic_id,
    title,
    description,
    acceptance_criteria,
    status,
    priority,
    assignee_id,
    reporter_id,
    sprint_id,
    story_points,
    created_at,
    updated_at,
    deleted_at
FROM user_stories
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL
LIMIT 1;

-- name: CheckUserStoryReferences :one
SELECT
    (sqlc.arg(epic_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM epics e
        WHERE e.id = sqlc.arg(epic_id)::uuid
          AND e.workspace_id = sqlc.arg(workspace_id)::uuid
          AND e.deleted_at IS NULL
    )) AS epic_valid,
    (sqlc.arg(sprint_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM sprints s
        WHERE s.id = sqlc.arg(sprint_id)::uuid
          AND s.workspace_id = sqlc.arg(workspace_id)::uuid
          AND s.status IN ('planned', 'active')
    )) AS sprint_valid;

-- name: UpdateUserStory :one
UPDATE user_stories
SET
    epic_id = $3,
    title = $4,
    description = $5,
    acceptance_criteria = $6,
    status = $7,
    priority = $8,
    assignee_id = $9,
    sprint_id = $10,
    story_points = $11,
    updated_at = NOW()
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL
RETURNING
    id,
    workspace_id,
    epic_id,
    title,
    description,
    acceptance_criteria,
    status,
    priority,
    assignee_id,
    reporter_id,
    sprint_id,
    story_points,
    created_at,
    updated_at,
    deleted_at;

-- name: SoftDeleteUserStory :one
UPDATE user_stories
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL
RETURNING id, workspace_id, title;

-- name: CreateSprint :one
INSERT INTO sprints (
    workspace_id,
    name,
    goal,
    status,
    start_date,
    end_date
)
VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING
    id,
    workspace_id,
    name,
    goal,
    status,
    start_date,
    end_date,
    created_at,
    updated_at,
    completed_at;

-- name: ListSprintsForWorkspace :many
SELECT
    id,
    workspace_id,
    name,
    goal,
    status,
    start_date,
    end_date,
    created_at,
    updated_at,
    completed_at
FROM sprints
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: GetWorkItem :one
SELECT
    id,
    workspace_id,
    user_story_id,
    parent_id,
    title,
    description,
    item_type,
    status,
    priority,
    assignee_id,
    reporter_id,
    sprint_id,
    due_date,
    story_points,
    created_at,
    updated_at,
    deleted_at
FROM work_items
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL
LIMIT 1;

-- name: CheckWorkItemReferences :one
SELECT
    (sqlc.arg(user_story_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM user_stories us
        WHERE us.id = sqlc.arg(user_story_id)::uuid
          AND us.workspace_id = sqlc.arg(workspace_id)::uuid
          AND us.deleted_at IS NULL
    )) AS story_valid,
    (sqlc.arg(sprint_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM sprints s
        WHERE s.id = sqlc.arg(sprint_id)::uuid
          AND s.workspace_id = sqlc.arg(workspace_id)::uuid
          AND s.status IN ('planned', 'active')
    )) AS sprint_valid,
    (sqlc.arg(parent_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM work_items wi
        WHERE wi.id = sqlc.arg(parent_id)::uuid
          AND wi.workspace_id = sqlc.arg(workspace_id)::uuid
          AND wi.deleted_at IS NULL
          AND wi.item_type IN ('task', 'bug')
    )) AS parent_valid;

-- name: UpdateWorkItem :one
UPDATE work_items
SET
    user_story_id = $3,
    parent_id = $4,
    title = $5,
    description = $6,
    item_type = $7,
    status = $8,
    priority = $9,
    assignee_id = $10,
    sprint_id = $11,
    due_date = $12,
    story_points = $13,
    updated_at = NOW()
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL
RETURNING
    id,
    workspace_id,
    user_story_id,
    parent_id,
    title,
    description,
    item_type,
    status,
    priority,
    assignee_id,
    reporter_id,
    sprint_id,
    due_date,
    story_points,
    created_at,
    updated_at,
    deleted_at;

-- name: SoftDeleteWorkItem :one
UPDATE work_items
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1
  AND workspace_id = $2
  AND deleted_at IS NULL
RETURNING id, workspace_id, title;

-- name: WorkItemHasActiveChildren :one
SELECT EXISTS (
    SELECT 1
    FROM work_items
    WHERE parent_id = $1
      AND workspace_id = $2
      AND deleted_at IS NULL
);

-- name: UserStoryHasActiveWorkItems :one
SELECT EXISTS (
    SELECT 1
    FROM work_items
    WHERE user_story_id = $1
      AND workspace_id = $2
      AND deleted_at IS NULL
);

-- name: CreateWorkItem :one
INSERT INTO work_items (
    workspace_id,
    user_story_id,
    parent_id,
    title,
    description,
    item_type,
    status,
    priority,
    assignee_id,
    reporter_id,
    sprint_id,
    due_date,
    story_points
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
RETURNING
    id,
    workspace_id,
    user_story_id,
    parent_id,
    title,
    description,
    item_type,
    status,
    priority,
    assignee_id,
    reporter_id,
    sprint_id,
    due_date,
    story_points,
    created_at,
    updated_at,
    deleted_at;

-- name: ListWorkItemsForWorkspace :many
SELECT
    id,
    workspace_id,
    user_story_id,
    parent_id,
    title,
    description,
    item_type,
    status,
    priority,
    assignee_id,
    reporter_id,
    sprint_id,
    due_date,
    story_points,
    created_at,
    updated_at,
    deleted_at
FROM work_items
WHERE workspace_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: CreateLabel :one
INSERT INTO labels (
    workspace_id,
    name,
    color
)
VALUES (
    $1, $2, $3
)
RETURNING
    id,
    workspace_id,
    name,
    color,
    created_at;

-- name: AddWorkItemLabel :one
INSERT INTO work_item_labels (
    work_item_id,
    label_id
)
VALUES (
    $1, $2
)
RETURNING
    work_item_id,
    label_id,
    created_at;

-- name: CheckWorkItemLabelReferences :one
SELECT
    EXISTS (
        SELECT 1 FROM work_items wi
        WHERE wi.id = sqlc.arg(work_item_id)::uuid
          AND wi.workspace_id = sqlc.arg(workspace_id)::uuid
          AND wi.deleted_at IS NULL
    ) AS work_item_valid,
    EXISTS (
        SELECT 1 FROM labels l
        WHERE l.id = sqlc.arg(label_id)::uuid
          AND l.workspace_id = sqlc.arg(workspace_id)::uuid
    ) AS label_valid;

-- name: RemoveWorkItemLabel :one
DELETE FROM work_item_labels wil
USING work_items wi, labels l
WHERE wil.work_item_id = sqlc.arg(work_item_id)::uuid
  AND wil.label_id = sqlc.arg(label_id)::uuid
  AND wi.id = wil.work_item_id
  AND wi.workspace_id = sqlc.arg(workspace_id)::uuid
  AND wi.deleted_at IS NULL
  AND l.id = wil.label_id
  AND l.workspace_id = sqlc.arg(workspace_id)::uuid
RETURNING wil.work_item_id, wil.label_id, wil.created_at;

-- name: ListLabelsForWorkItem :many
SELECT l.id, l.workspace_id, l.name, l.color, l.created_at
FROM labels l
JOIN work_item_labels wil ON wil.label_id = l.id
JOIN work_items wi ON wi.id = wil.work_item_id
WHERE wi.id = sqlc.arg(work_item_id)::uuid
  AND wi.workspace_id = sqlc.arg(workspace_id)::uuid
  AND wi.deleted_at IS NULL
  AND l.workspace_id = wi.workspace_id
ORDER BY l.name;
