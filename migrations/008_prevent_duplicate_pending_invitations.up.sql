CREATE UNIQUE INDEX idx_workspace_invitations_pending_unique_email
ON workspace_invitations (workspace_id, LOWER(email))
WHERE status = 'pending'
  AND email IS NOT NULL;