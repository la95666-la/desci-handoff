# Fair Launch V1 dispatcher architecture record

The original monolithic implementation could not satisfy EIP-170. The approved immutable dispatcher/facet architecture resolves that blocker while preserving the proxy-facing aggregate ABI and ERC-7201 namespace/layout.

Measured with the frozen compiler baseline (`solc 0.8.22`, viaIR, optimizer):

- Empty mechanical specimen runtime: 10,241 bytes.
- Full implementation before external helpers: 26,149 bytes.
- With stateless pure helper library: 26,625 bytes.
- With additional approved read-only STATICCALL runtime library: 27,439 bytes.
- BSC/EVM EIP-170 maximum: 24,576 bytes.
- Requested operating target: at most 23,000 bytes.

External library calls increased runtime because ABI marshalling and linked-call
stubs were larger than the short validation routines removed. The frozen ABI
contains 169 ABI entries (57 owned functions, 32 owned events and 56 owned
errors plus inherited UUPS/Ownable ABI), so selector dispatch and return-data
encoding account for a substantial irreducible portion of runtime.

Resolved architecture: one UUPS dispatcher with immutable facet addresses; five fixed facets share the reviewer-owned storage definition and execute under delegatecall. There is no mutable selector table or facet admin. Every routing change requires a complete new dispatcher/facet set and normal UUPS governance. Measured runtimes: dispatcher 4,757; core 10,073; release 4,250; funds 5,350; admin 7,416; views 7,859 bytes. All are below the 23,000-byte policy target without unlimited-size test configuration. Deployment remains unauthorized until external gates pass.

Production reviewer authority provisioning is also fail-closed. All authority, key, state, current approval, and previous approval files must be regular single-link files below the fixed protected machine root. Every directory from that root to each leaf parent must remain privileged-owned and non-writable by untrusted principals. Unix validation requires root-owned, non-group/world-writable directories and leaf files and reads leaves through one `O_NOFOLLOW` descriptor with stable `fstat` identity. Windows validation uses only `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`, validates complete ACL JSON, rejects reparse points and untrusted write/create/delete/control rights on every directory and leaf, and requires stable same-handle and pathname identity through the read. If the runtime or filesystem cannot prove these properties, production authorization fails closed; provisioning must establish the required ownership/ACLs rather than bypass the validator.
