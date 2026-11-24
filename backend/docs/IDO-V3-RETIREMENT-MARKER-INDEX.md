# Indexed retirement marker lookup

Source-only follow-up to bf06ab7. Production enable attempt was rolled back:
all six historical enrollments/journals were valid but COUNT(JSON_CONTAINS)
scanned ordinary immutable coverage history (~250k rows /796MiB at observation).
Repeated archived projects repeated that cost inside90/100s audit budgets.
Deadline was reported as enrollment failure; no evidence was committed.

## Fix

Independent migration `2026_10_31_ido_v3_retirement_marker_index.sql` adds:

- VIRTUAL generated `retirement_marker = JSON_CONTAINS_PATH(payload,'one','$.RetiredTests')`;
- `ix_v3_coverage_retirement_marker(chain_id,proxy_address,contract_version,retirement_marker,epoch,block_number)`;
- `ix_v3_retired_latest(submission_id,id)` for current enrollment/anti-newer lookup;
- replacement exemption view retaining all original predicates, but evaluating
  JSON membership only for indexed V3 marker envelopes.

Go idle/active/enrollment audit uses the same indexed marker restriction,
retains exact COUNT=1 semantics across immutable epochs, and fails closed on
missing migration/index, duplicate/foreign markers or SQL errors. Index is NOT
retirement authority: original exact JSON project membership, canonical
receipt/journal, archived identity, enrolled-not-revoked state, consumed nonce,
short TTL and exact active/idle fence rules are unchanged. No row is manually
marked retired and no old payload/digest/expiry is rewritten.

Maintenance now preserves finite `retired_marker_lookup` versus
`retired_marker_membership` errors. A context timeout during input preparation
is `sender_maintenance_deadline`, not a claimed enrollment revocation.

## Rollout prerequisite (not executed)

Maintenance remains OFF in production after rollback. Back up exact schema,
view definitions and affected evidence, verify identity and ledger, restore on
network-none MySQL8.4, then independently authorize migration. Index creation
necessarily reads existing payloads once and consumes disk/IO; it is not free
or an instant-only DDL claim. Check metadata lock risk, headroom and replication
before apply. Do not execute while allocators depend on a partially changed
view. The migration is checksum-bound and approval-gated, additive, repeatable;
partial failure is an incident requiring actual schema/ledger reconciliation.

Install migration BEFORE application code or re-enabling sender maintenance.
Verify generated expression/index columns and live EXPLAIN plans for Go lookup,
latest evidence, exemption view and nonce trigger path. Missing index causes
explicit rejection, never a fallback to a whole payload scan. New column is
generated; existing producer INSERTs name columns and remain compatible. No
new database data transformation or evidence refresh is hidden in migration.

After separate rollout, enable maintenance, observe real dual-provider audit
and SQL commit latency, fresh exemptions and sender ready; only then user may
recheck Set deadline/time window and retry a normal create. No chain operation
is authorized by this source change. Backend/Job/SQL/config gates stay separate.

Rollback: disable maintenance and keep the additive generated/index structures;
application rollback is compatible. Restore the previous view only under an
independent reviewed operation if required (would restore slow scans). Do not
drop referenced indexes, delete evidence, alter original nonce slots or fake
sender readiness. The SQL nonce triggers are not relaxed/replaced here.

## Validation

network-none MySQL8.4 runner installs the exact independent migration twice,
rejects missing approval, retains expiry/revocation/nonce/one-shot tests, adds
20k quiet1KiB coverage rows, verifies marker lookup AND exemption view EXPLAIN
use the marker index with <=4 estimated marker rows, compares old/new exact
membership, rejects foreign/duplicate markers and preserves sender guards.
Unit tests include missing index/schema/SQL errors and secret-safe stages;
race/vet and Linux builds are required before packaging.
