# V3 API 安全错误诊断（第一阶段）

## 契约

保留既有顶层 `code` 和失败 HTTP 状态，增加：

```json
{
  "status": false,
  "code": "PROJECT_CREATE_ONCHAIN_FAILED",
  "reason_code": "CREATE_DEADLINE_EXPIRED",
  "stage": "signature_persist",
  "message": "固定、脱敏的原因说明",
  "next_action": "refresh_and_check",
  "submission_outcome": "unsigned_cancelled",
  "submission_id": "65",
  "request_id": "24位十六进制排查编号"
}
```

- 原因与阶段来自有限枚举；未知原因保持 `UNAVAILABLE`，不根据含糊文本猜测 RPC 故障或真实重组。
- `request_id` 由服务端生成，并关联本次 API 拒绝日志和 `X-Request-ID`；不是客户端输入，也不是旧日志已有编号。
- 保留原观察流程的有限 `check` 细节；不返回 provider URL、SQL、raw_tx、签名、配置或底层错误正文。
- ID 使用十进制字符串，hash 仅使用已持久化且格式正确的原交易哈希。
- 不自动重试任何 POST、签名、广播、Safe 服务提交，不改变资金/审批/nonce 门禁。

## 本次覆盖

1. 创建：Set 读取、截止时间、身份/preflight、intent、unsigned 检查、签名前准备、签名后复核和原交易广播阶段。
2. 生命周期：保留已有储备不足和权限错误；其他 V3 错误返回阶段/原 attempt 状态。
3. 签名前公共守卫：区分 nonce 对齐、非退役 suspended sender、退役历史证据对账未通过；不把节点读失败武断说成已确认 nonce 冲突。
4. 白名单候选/status/提交/二审/handoff/proposal：沿调用链保留更具体的下层原因和阶段，不由控制器统一吞掉。
5. 二审写入点：同人二审、已过审批时间、权限失败可返回明确原因；没有明确分类的并发/内容问题仍报告阶段，不猜测。
6. Safe 回执：复用既有 phase/check，不把所有失败都显示为普通未知回执。
7. Admin：全局错误映射和白名单局部弹窗使用同一有限映射；status 失败留在页面；禁止 raw message 回退泄露 URL/密码等。

## 写结果的不确定性

创建/lifecycle 失败后以独立、最多3秒的 DB 读取核对已分配 submission：

| outcome | 证据与提示 |
|---|---|
| `unsigned_cancelled` | 必须读回 cancelled，nonce/hash/raw_tx 全空且 broadcast_count=0。只说明此次未广播，不保证再次提交会通过 |
| `signed_or_submitted` | 有 nonce/hash/raw bytes 或广播记录。核对原 attempt/hash，绝不自动重发 |
| `unknown` | DB 不可读、结果不明或尚未确认取消。不能声称“没有签名/没有执行” |

已有 submission 而不是确认 unsigned_cancelled 时，next_action 为
`reconcile_original_do_not_resend`。未知 HTTP/审批 POST 仍走原列表读回机制，
不会因为显示了具体原因就解除页面 unknown 防重复提交保护。

过截止时间是业务事实；若同时已有签名记录，必须仍提示对账，不可只给“改时间再试”。

## 验证与边界

- unit/race：有限原因/阶段、嵌套原因保留、HTTP 原码兼容、随机排查编号、原始敏感错误不泄漏。
- DB mocks：unsigned cancelled、signed、submitted、ready、DB故障；只有取消且无签名证据才允许“未签名取消”表述。
- 现有创建、生命周期、policy、签名/outbox 测试保持通过；真实 disposable MySQL 验证退役守卫/trigger/回滚规则没有被绕过。
- Admin：诊断映射、恶意阶段/未知码、签名结果未知即使HTTP400也视为歧义、既有只读/审批界面测试。

这不是“一次性让所有 V3 分支都有精确根因”。底层仍有把 RPC/DB 错误统一隐藏的历史 helper，
本次只能返回确切失败阶段及未知原因；后续应继续在源头补 typed errors，而不是靠报错文本猜测。
Set/runtime registration、所有代币准入与 C 端钱包本地 pending 并非本次完全覆盖范围。
本次不修改白名单性能计划中的授权边界，不改schema，不自动打包部署，不执行生产写操作。
