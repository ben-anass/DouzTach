DROP INDEX IF EXISTS idx_sprints_one_active_per_workspace;

ALTER TABLE user_stories
    ADD COLUMN sprint_id UUID REFERENCES sprints(id) ON DELETE SET NULL;

UPDATE user_stories us
SET sprint_id = ss.sprint_id
FROM sprint_stories ss
WHERE ss.story_id = us.id;

DROP INDEX IF EXISTS idx_sprint_stories_story_id;

ALTER TABLE user_stories
    DROP COLUMN backlog_position;
