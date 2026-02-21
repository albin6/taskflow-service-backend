-- Allow creating databases even if we are not superuser (some restrictions apply)
-- These commands should be run by a user with CREATEDB and CREATEROLE privileges (usually 'postgres')

-- 1. Create Users
DO
$do$
BEGIN
   IF NOT EXISTS (
      SELECT FROM pg_catalog.pg_roles
      WHERE  rolname = 'taskflow') THEN
      CREATE USER taskflow WITH PASSWORD 'taskflow_dev_password';
   END IF;
   
   IF NOT EXISTS (
      SELECT FROM pg_catalog.pg_roles
      WHERE  rolname = 'api_user') THEN
      CREATE USER api_user WITH PASSWORD 'api_password';
   END IF;
END
$do$;

-- 2. Grant Privileges (Safe to run multiple times)
ALTER USER taskflow CREATEDB;
ALTER USER api_user CREATEDB;

-- 3. Create Databases (Verify existence first is tricky in pure SQL script, better run separately or ignore error)
-- Note: You cannot run CREATE DATABASE inside a transaction block.
-- Please run these lines individually if the script fails, or use `psql -f`

-- CREATE DATABASE taskflow OWNER taskflow;
-- CREATE DATABASE api_db OWNER api_user;
