BEGIN;

-- Rename the old name column to nickname.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'users'
          AND column_name = 'name'
    )
    AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'users'
          AND column_name = 'nickname'
    ) THEN
        ALTER TABLE users RENAME COLUMN name TO nickname;
    END IF;
END $$;

-- Add columns required by the new user model.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS username VARCHAR(50),
    ADD COLUMN IF NOT EXISTS nickname VARCHAR(100),
    ADD COLUMN IF NOT EXISTS favorite_language VARCHAR(100),
    ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

-- Generate usernames for existing users without changing existing usernames.
UPDATE users
SET username = 'user_' || REPLACE(id::text, '-', '')
WHERE username IS NULL OR username = '';

-- Ensure required fields are populated.
UPDATE users
SET nickname = COALESCE(NULLIF(nickname, ''), 'User')
WHERE nickname IS NULL OR nickname = '';

ALTER TABLE users
    ALTER COLUMN username SET NOT NULL,
    ALTER COLUMN nickname SET NOT NULL;

-- Add a unique index for usernames.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username
    ON users (username);

COMMIT;
