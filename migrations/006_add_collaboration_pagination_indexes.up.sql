CREATE INDEX idx_chat_messages_channel_created_at
    ON chat_messages (channel_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_notifications_recipient_created_at
    ON notifications (recipient_user_id, created_at DESC, id DESC);

CREATE INDEX idx_activity_logs_workspace_created_at
    ON activity_logs (workspace_id, created_at DESC, id DESC);
