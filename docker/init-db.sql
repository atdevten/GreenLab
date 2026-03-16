-- GreenLab IoT — per-service database initialisation
-- Runs automatically on first postgres container start via docker-entrypoint-initdb.d.
-- The default 'greenlab' database is already created by POSTGRES_DB; we create
-- the service-specific databases here and grant the greenlab user full access.

CREATE DATABASE iam_db;
CREATE DATABASE device_registry_db;
CREATE DATABASE alert_db;
CREATE DATABASE supporting_db;

GRANT ALL PRIVILEGES ON DATABASE iam_db            TO greenlab;
GRANT ALL PRIVILEGES ON DATABASE device_registry_db TO greenlab;
GRANT ALL PRIVILEGES ON DATABASE alert_db           TO greenlab;
GRANT ALL PRIVILEGES ON DATABASE supporting_db      TO greenlab;
