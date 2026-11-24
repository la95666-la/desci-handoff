# V3 public stream throughput / lock scope

## Failure addressed

Normal project ticks previously locked `ido_v3_proxy_stream` before original
creation admission and retained it across consumer live RPC. The independently
scheduled producer needed that same row to commit. Successful batches taking
over four seconds also halved the next batch, amplifying fixed RPC overhead.

## Runtime change

- Running-project admission runs outside the outer stream writer transaction.
  Bootstrap and halted-project recovery retain their controlled serialization.
- Consumer work is prepared against a request-local ordered RPC transcript.
  A missing observation aborts the tentative SQL transaction. Only **after
  rollback** does the missing live RPC execute. Tentative SQL writes never
  commit. Subsequent probes replay observations already fetched and stop at
  the next missing one. Each independent source observation remains a separate
  transcript entry; this is not a cross-request canonicality cache.
- This rollback boundary matters: MySQL triggers / foreign keys can acquire
  public-row locks even without an explicit stream `FOR UPDATE`.
- Publication takes the stream lock, verifies mode/state/epoch/anchor,
  monotonic producer revision/height and the prepared immutable prefix, and
  requires the exact original project cursor/hash/revision/state. It replays
  the full transcript without network. Different RPC arguments/order, missing
  entries or unconsumed evidence reject publication. Scheduling decisions are
  replayed too, so different CPU/SQL elapsed times cannot expand the prefix.
- Live confirmation/canonical anchors are checked again after publication,
  outside the stream lock. Failure revokes the exact published project revision;
  failed revocation is explicitly reported. Original financial, journal,
  receipt, checkpoint and outbox transactions remain in use.

## Producer sizing

Start at four blocks. Successful sub-six-second units grow small batches up
to sixteen; larger units grow only below three seconds, capped at 32. Normal
success never shrinks the batch. Actual unit deadline exhaustion still shrinks
only after fresh durable-anchor/runtime verification. Avoid starting a full
unit when the parent turn has less than a unit budget plus one second left.
Provider log ranges remain at most five blocks, with four scanner requests in
flight; no fixed scanner delay or write retry is added.

## Validation and operational limits

`scripts/test-v3-proxy-stream-mysql.sh` uses the network-none disposable MySQL
8.4 fixture. A committed synthetic scope uses another connection to acquire
the public row with `FOR UPDATE NOWAIT` during live RPC. It proves producer
advancement during preparation, consumer publication of only its prepared
prefix, epoch-change rejection and post-publication lost-depth revocation.
Existing financial-event/idempotence/reorg/crash/retained-epoch tests remain.

Preparation repeats SQL computation and rolls it back. This trades extra
bounded SQL work for eliminating network-length public locks; it does not
eliminate project-row contention or reduce all admission RPC. Preparation is
bounded by the caller deadline, 128 blocks and 4096 observation probes.
Bootstrap/recovery and the producer's own final live fences can still hold the
public row. These are not claimed to be zero-lock or zero-cost paths.

No migration or configuration switch is required. Source tests are not a
production catch-up measurement. After a separately authorized Job release,
sample actual confirmed head and producer/consumer cursors over multiple turns,
RPC throttling/errors and lock waits. Acceptance requires falling lag and
intact business effects, not merely container health or recent success time.
