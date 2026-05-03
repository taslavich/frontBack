ALTER SYSTEM SET timezone = 'UTC';

ALTER DATABASE twinbid SET timezone = 'UTC';

ALTER ROLE taslav SET timezone = 'UTC';

SELECT pg_reload_conf();