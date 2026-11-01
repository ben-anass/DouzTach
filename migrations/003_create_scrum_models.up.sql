CREATE TABLE epics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    description TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'completed', 'archived')),
    priority VARCHAR(20) NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    color VARCHAR(30),
    start_date DATE,
    end_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE sprints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    goal TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'completed', 'cancelled')),
    start_date DATE,
    end_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    CHECK (start_date IS NULL OR end_date IS NULL OR start_date <= end_date)
);

CREATE TABLE user_stories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    epic_id UUID REFERENCES epics(id) ON DELETE SET NULL,
    title VARCHAR(200) NOT NULL,
    description TEXT,
    acceptance_criteria TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'to_do' CHECK (status IN ('to_do', 'in_progress', 'in_review', 'done', 'blocked')),
    priority VARCHAR(20) NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    assignee_id UUID REFERENCES users(id) ON DELETE SET NULL,
    reporter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sprint_id UUID REFERENCES sprints(id) ON DELETE SET NULL,
    story_points INTEGER CHECK (story_points >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE sprint_stories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sprint_id UUID NOT NULL REFERENCES sprints(id) ON DELETE CASCADE,
    story_id UUID NOT NULL REFERENCES user_stories(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    position INTEGER NOT NULL DEFAULT 0,
    UNIQUE (sprint_id, story_id)
);

CREATE TABLE labels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(80) NOT NULL,
    color VARCHAR(30) NOT NULL DEFAULT '#7C3AED',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, name)
);

CREATE TABLE work_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_story_id UUID REFERENCES user_stories(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES work_items(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    description TEXT,
    item_type VARCHAR(20) NOT NULL CHECK (item_type IN ('task', 'bug', 'subtask')),
    status VARCHAR(20) NOT NULL DEFAULT 'to_do' CHECK (status IN ('to_do', 'in_progress', 'in_review', 'done', 'blocked')),
    priority VARCHAR(20) NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    assignee_id UUID REFERENCES users(id) ON DELETE SET NULL,
    reporter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sprint_id UUID REFERENCES sprints(id) ON DELETE SET NULL,
    due_date DATE,
    story_points INTEGER CHECK (story_points >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE work_item_labels (
    work_item_id UUID NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
    label_id UUID NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (work_item_id, label_id)
);

CREATE INDEX idx_epics_workspace_id ON epics (workspace_id);
CREATE INDEX idx_user_stories_workspace_id ON user_stories (workspace_id);
CREATE INDEX idx_user_stories_epic_id ON user_stories (epic_id);
CREATE INDEX idx_user_stories_assignee_id ON user_stories (assignee_id);
CREATE INDEX idx_sprints_workspace_id ON sprints (workspace_id);
CREATE INDEX idx_work_items_workspace_id ON work_items (workspace_id);
CREATE INDEX idx_work_items_sprint_id ON work_items (sprint_id);
CREATE INDEX idx_work_items_user_story_id ON work_items (user_story_id);
CREATE INDEX idx_work_items_parent_id ON work_items (parent_id);
CREATE INDEX idx_work_item_labels_label_id ON work_item_labels (label_id);
