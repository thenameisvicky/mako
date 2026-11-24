-- 006_drop_retired_tables.sql
-- Drop the tables left behind by features Mako no longer has:
--
--   policies, budgets, usage_records  team policy, token budgets and usage
--                                     recording (removed in v0.5.0-alpha; their
--                                     migrations 003 and 005 were deleted)
--   credentials                       shared provider-key storage (credential
--                                     swapping removed in v0.3.0-alpha)
--
-- DESTRUCTIVE: any rows in these tables are deleted. Back them up first if you
-- want to keep them. Idempotent, so re-running cmd/migrate is safe; their
-- indexes are dropped along with the tables.

DROP TABLE IF EXISTS usage_records;
DROP TABLE IF EXISTS budgets;
DROP TABLE IF EXISTS policies;
DROP TABLE IF EXISTS credentials;
