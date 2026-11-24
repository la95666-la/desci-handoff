# Fair Launch RFC 8785 mechanical evidence

This review-only tree freezes eight detached-record schemas: the §18.4 payload families, an explicit
deployment attestation, a key-revocation record, and their signature envelope. It contains canonical SHA-256
vectors, deterministic real Ed25519 2-of-3 keys/signatures, RFC 8785 edge
vectors, domain/key-set/threshold binding, frozen capability-set checks and
cross-record bootstrap/watermark/deployment/recovery bindings. Every payload family has a
real detached threshold-signature vector; transition fixtures cover digest
chains, rollback rejection, typed joins and identity conflicts.

The key-set schema includes `signaturesByPreviousKeySet`; the genesis vector is
empty because it has no predecessor. A second deterministic key set is genuinely
signed 2-of-3 by the predecessor key set. The verifier executes positive rotation,
wrong-digest, insufficient/unknown signer, tamper, validity-window and revocation
negative vectors. The bootstrap root digest is the SHA-256 of the actual canonical
genesis key-set bytes.

No payload or signature envelope belongs inside the runtime tarball. Signature
bytes cover the exact canonical UTF-8 payload bytes only. The payload contains
its own fixed domain; no hidden prefix is used. The included `artifact.bin` is a
format fixture, not a production archive attestation. The restore vector binds
to the separate canonical deployment-attestation digest, not to the image manifest.
Typed-join vectors bind `implementationCodeHash` and `sourceRegistryRevision` and
fail closed on identity conflict or source-revision regression.

Run `node tools/verify.js .` from this directory, or use the repository-wide
`bash scripts/tests/fair_launch_mechanical_evidence_test.sh`.
