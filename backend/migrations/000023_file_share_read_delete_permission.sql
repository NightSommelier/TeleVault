ALTER TABLE file_shares
DROP CONSTRAINT IF EXISTS file_shares_permission_check;

ALTER TABLE file_shares
ADD CONSTRAINT file_shares_permission_check CHECK (permission IN ('read', 'read_delete'));

---- create above / drop below ----

ALTER TABLE file_shares
DROP CONSTRAINT IF EXISTS file_shares_permission_check;

ALTER TABLE file_shares
ADD CONSTRAINT file_shares_permission_check CHECK (permission IN ('read'));
