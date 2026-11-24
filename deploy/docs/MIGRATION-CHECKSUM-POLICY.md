# Migration checksum compatibility policy

## 2026-08-30 security migration

The canonical SHA-256 for `2026_08_30_security_control_plane_and_event_fence.sql` is:

```text
2401bf1d558e125406e59b4bb76e3ad4acb724ba60de8867b979114e25c9b1e7
```

The earlier checksum `4ee32387ab98d9ffd061a9eef6a58582e5ed8b5fcaf80872190160cb4b4ea20a` used a postflight predicate that rejected MySQL's semantically equivalent `tinyint(1)` metadata for `user.is_enable`. A clean sequential migration could not finish with that file.

Before deploying the corrected migration to any shared environment, inspect `schema_migrations`:

- no row: deploy the canonical migration normally;
- old checksum with `status='applying'`: stop and manually inspect the partially applied DDL; never replay automatically;
- old checksum with `status='applied'`: stop and perform a compatibility review before changing the ledger; do not overwrite the checksum or pretend the new file ran;
- canonical checksum with `status='applied'`: the normal checksum gate may continue.

Repository history proves only that the old file was introduced in the current baseline commit. It does not prove whether an external Testnet or production database executed it.

Local preflight:

```bash
JOINCARE_BACKEND_ROOT=/absolute/path/to/joincare_server-master \
  bash scripts/tests/mysql_security_integration_test.sh all
```

The command accepts only disposable `joincare_reorg_verify_*` databases, rebuilds before each destructive worker test, and removes its temporary database/user by default. Set `JOINCARE_KEEP_TEST_DB=1` only when preserving the final evidence database for inspection.
