INSERT INTO sprint_stories (sprint_id, story_id, position)
SELECT
    sprint_id,
    id,
    (ROW_NUMBER() OVER (PARTITION BY sprint_id ORDER BY created_at, id) - 1)::INTEGER
FROM user_stories
WHERE sprint_id IS NOT NULL
ON CONFLICT (sprint_id, story_id) DO NOTHING;

WITH ranked_memberships AS (
    SELECT
        ss.id,
        ROW_NUMBER() OVER (
            PARTITION BY ss.story_id
            ORDER BY
                (ss.sprint_id = us.sprint_id) DESC NULLS LAST,
                ss.added_at,
                ss.id
        ) AS membership_rank
    FROM sprint_stories ss
    LEFT JOIN user_stories us ON us.id = ss.story_id
)
DELETE FROM sprint_stories ss
USING ranked_memberships rm
WHERE ss.id = rm.id
  AND rm.membership_rank > 1;

CREATE UNIQUE INDEX idx_sprint_stories_story_id
    ON sprint_stories (story_id);

ALTER TABLE user_stories
    ADD COLUMN backlog_position INTEGER NOT NULL DEFAULT 0;

ALTER TABLE user_stories
    DROP COLUMN sprint_id;

CREATE UNIQUE INDEX idx_sprints_one_active_per_workspace
    ON sprints (workspace_id)
    WHERE status = 'active';
