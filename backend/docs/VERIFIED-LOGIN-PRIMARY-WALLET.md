# Verified login establishes the primary wallet binding

Source/test fix only. No production SQL, migration, configuration activation,
release, wallet signature or purchase transaction is performed by this change.

## Defect

Signed registration/login created or authenticated `user` and the non-recyclable
`user_wallet_identity`, but did not ensure the verified `user_wallet` row required
by the V3 membership-proof ownership join. Visiting GET `/client/auth/v1/wallets`
could lazily create it, making purchase eligibility depend on a prior page visit.
Re-signing login did not repair the missing row. Membership and ownership are
separate; this fix does not change approved lists or Merkle roots.

## Change

After existing signature/challenge verification, signed login locks the user and
completes binding restoration plus last-login update in ONE transaction. New
registration and re-registration of an existing enabled account use the same
binding invariant. A login response/token is built only after commit succeeds.

The verified signer must equal the locked user's current primary wallet. Require
an existing exact lowercase EVM immutable ownership claim for that UID; no claim
is inserted/reassigned by the helper. Account disable/session identity failures,
missing/foreign claims, foreign binding or duplicate/ambiguous rows fail closed.

Missing binding is inserted explicitly (NOT ON DUPLICATE KEY UPDATE, which could
target a foreign verified-wallet unique collision). A same-user legacy unverified
row is upgraded. An existing verified row retains verified_at and method, including
personal_sign; ordinary login does not slide verification time. All prior binding
history remains. Primary flag normalization and the proof ownership join readback
complete before commit. Database ownership triggers and unique indexes remain.

Only actual signed login/registration invoke this helper. AutoLogin token renewal,
profile/proof reads and arbitrary posted UID/address cannot mint verification.
Existing wallet-list compatibility behavior is unchanged; purchase no longer
depends on it after a successful signed login on the fixed Backend.

No schema change or bulk backfill needed. Existing affected users repair through
normal signed login after separately authorized deployment. A failed DB operation
requires a fresh login challenge as before; no automatic nonce replay is added.
This does not grant a proof when the address is absent from the executed list,
the session mismatches or other proof/contract conditions fail.

## Validation

Unit: new binding, verified binding, legacy row, missing/foreign identity,
foreign/ambiguous binding, write/readback failure, disabled account, wrong signer,
signature-before-restoration ordering and no token-renewal authority.

Network-none MySQL8.4: exact missing-binding shape for the reported address,
production-shaped generated verified-wallet uniqueness and owner INSERT trigger,
proof ownership join succeeds, eight concurrent logins leave one row, timestamps
do not slide, registration rollback leaves no binding, legacy repair and foreign/
missing identity refusal. Fixture schema is isolated and deleted, never production.

Scope: Backend only; no frontend/Admin/Job behavior or chain/nonce/outbox change.
Includes60f8bb9 ancestor (light Safe export/proposal and display optimization).
