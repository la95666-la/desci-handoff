# V3 sender readiness before submission

Source/test only, no production release/config/chain operation.

Projects list displays read-only sender state. Polling reads only the existing
GET /admin/v1/ido-v3/sender-readiness (10s while unavailable,30s when ready).
Shows evidence remaining/request threshold if supported by Backend, and exact
original submission ID for an active fence. Unknown, revoked, permission and
unresolved states never become ready merely because evidence time is positive.

V3 create/activate/end/platform-fee confirmation re-reads readiness before calling
the write endpoint. In-flight read refuses cached-ready use. No operation auto
starts after a poll succeeds, no failed transaction is replayed, no extra nonce
or signature authority. Server still independently checks current authorization,
project state, gas/simulation, evidence and original nonce/outbox fences.

Companion Backend/Job runway patch retains180s/30s/270s reserves and300s TTL;
publishing Admin alone improves diagnostics but cannot fix backend maintenance.
All production upgrades remain separately authorized.
