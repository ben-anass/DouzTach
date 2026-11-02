ALTER TABLE workspace_invitations
DROP CONSTRAINT workspace_invitations_status_check;

ALTER TABLE workspace_invitations
ADD CONSTRAINT workspace_invitations_status_check
CHECK (
    status IN (
        'pending',
        'accepted',
        'rejected',
        'expired',
        'cancelled'
    )
);
