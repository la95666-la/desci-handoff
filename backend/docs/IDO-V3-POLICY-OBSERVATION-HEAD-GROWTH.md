# Policy observation permits normal head growth

The `current_policy` stage formerly required a second `eth_blockNumber` to
equal the height selected before policy/project/runtime reads. These reads take
multiple round trips; normal BSC growth therefore caused false refusal despite
a stable canonical header and matching policy, project and Safe execution.

The recording path now uses `checkApprovedV3ObservationCurrentPolicy`: select
one latest observed height, validate the exact policy version/enabled/root,
runtime/project identity and allowed state there, then independently recheck
that height's canonical hash. The second block-number equality read is removed.
Original Safe transaction/receipt checks, repeated receipt/log comparison,
creation/admission, scanner freshness and atomic DB gates are unchanged.

This is a pinned instantaneous observation, NOT proof that no policy change can
occur after the observed height. It does not send Safe transactions or authorize
wallets; proof issuance retains its own independent current-state checks.

Regression tests advance the simulated chain during header/contract reads and
require success without a second head-equality query. Wrong root/version/mode,
changed canonical hash, wrong project identity, terminal state, Active public
mode and RPC failures continue to reject. Backend only; no schema/config change.
