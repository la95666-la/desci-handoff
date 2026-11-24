# IDO 管理端错误展示约定

源码与测试改动，不代表线上已生效；不触发签名、交易、数据库修复或配置启用。

## 展示格式

中文原因／下一步 + 英文原因代码（reason_code）+ 英文接口代码（code，若不同）
+ 中文失败阶段（stage）+ 已证实的提交结果 + 原记录编号／哈希／排查编号。

示例：发送钱包历史证据待维护核验或剩余有效时间不足。
原因代码：SENDER_EVIDENCE_PENDING。接口错误码：PROJECT_ACTIVATE_ONCHAIN_FAILED。
失败阶段：发送钱包就绪检查（sender_readiness）。

代码必须满足有限格式，中文原因来自有限映射，不解析 RPC/SQL message 猜原因。
精确失败项尚未结构化时明确说明未知；不把 broad *_FAILED 翻译为已经链上回退。
无服务端代码时 IDO_RESULT_UNCONFIRMED 是前端诊断代码，不冒充合约错误码。
历史 last_error/result 文本无法可靠分类时使用 IDO_STORED_ERROR_UNCLASSIFIED，
请运维按原记录检索日志；不把任意 provider、SQL、URL、签名文本直接展示。

## 提交结果

- not_reserved：本次未建立新的提交记录。白名单审批也使用该字段，不一律声称
  “未签名未广播”或建议检查 sender；其他历史交易不在该结论范围内。
- unsigned_cancelled：已核实本次未签名并取消；不能跳过重新校验。
- signed_or_submitted：已签名或已提交，只核对原交易，勿重发。
- unknown：未确认，不推断失败／成功／可重试。
- reconcile_original_do_not_resend 优先于冲突的 not_reserved 提示。
- reason_code 不能单独证明未签名；sender evidence 也可能在签名后复查失败。

## 覆盖与限制

覆盖 Set 预览/提交、普通四眼审批、创建/激活/结束/取消/手续费、全局暂停、
白名单预览/状态/工单/二审/Safe/回执、代币准入/运行身份、跟踪/维护/归档、
交易运维及原签名继续入口。保留项目币储备 raw 金额精确换算与短缺说明。
GET 失败不证明不存在工单／交易；审批批准不等于链上业务完成。

Backend Set 预览新增有限失败阶段；保留 guard/simulation/runtime/审批/原 nonce
逻辑，不将 diagnostic preview 升级为执行授权。旧版和尚未结构化的错误使用
保守中文兜底，尚不能显示每个合约自定义 revert 的独立中文原因。

未进行生产错误复现（尤其不点击写入口），验收来自静态返回路径及离线测试。
生产发布后仍应只读核验实际版本和展示；没有凭测试断言线上已修复。

## Set 权限拒绝

预览及提交路由均要求 m2_project:write。ADMIN_PERM_DENIED 属于路由入口
权限拒绝，不应以“参数校验未通过”解释。Backend 额外返回结构化
required_permission，Admin 不解析 missing permission 文本。旧 Backend
只有代码时仍说明缺少权限，但不从原始 message 猜具体权限。

Set 表单初始及权限读取失败均不允许预览／提交；读取 my/permissions 并在
预览／提交前复查。只拥有 read/approve 的二审账号仍可查看，不授予 write。
Backend 每次请求重新检查有效角色／权限，前端判断不代替授权。
