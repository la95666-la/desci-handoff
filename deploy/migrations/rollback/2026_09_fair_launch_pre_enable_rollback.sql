-- DESTRUCTIVE ROLLBACK TEMPLATE. DO NOT RUN after any Fair Launch source/project/write.
-- Operator must first prove all blocker queries return 0 and take a verified backup.
SELECT 'fair_launch_sources' blocker,COUNT(*) blocker_count FROM chain_contract_source WHERE funding_mode='fair_launch';
SELECT 'fair_launch_projects' blocker,COUNT(*) blocker_count FROM projects WHERE funding_mode='fair_launch' OR contract_version='fair_launch_v1';
SELECT 'ce_v2_applications' blocker,COUNT(*) blocker_count FROM chain_event_application_attempt_v2;
SELECT 'shadow_ledger' blocker,COUNT(*) blocker_count FROM fund_ledger_v2_shadow;
SELECT 'signing_journal' blocker,COUNT(*) blocker_count FROM fair_launch_gateway_signing_journal;
-- If and only if every count above is zero, an approved manual rollback may drop
-- the additive Fair Launch tables/views/constraints/columns in reverse migration order.
-- No DROP statement is included to prevent accidental execution or automation.
