# IDO 管理错误语义修正

仅源码及测试，无生产发布、权限变更、SQL migration 或链上操作。

- sender evidence reason 不声称未签名：它也可能来自签名后的复查。
  签名／广播结论须依赖 durable outcome、原记录及 next_action。
- Set 预览返回有限 project_admission/rpc_setup/confirmed_snapshot/
  governance_identity/project_params/governance 阶段；不将底层 Error() 序列化。
  初始 RPC 错误保留原因链，其他已折叠错误不捏造具体 RPC 故障。
- 代币 Safe 提案错误使用已有结构化 diagnostic，结果未知不能当成未提案。
- RBAC 403 附加 required_permission，原 code、message 与权限规则保持兼容。
  无权限请求仍在 controller 之前拒绝，不因诊断改动通过检查。

配套 Admin 显示中文含义、英文接口／原因／阶段代码，以及已证实的提交状态。
未知失败使用保守说明；不承诺每个合约 revert 已完成结构化。

生产只读排查发现 ido_approver（id3）仅 approver 角色，具有项目 read/approve
而非 write；Set 预览返回403符合路由策略，不是参数错误。未授予额外权限。
独立二审与发起人分离不变。另一个 superadmin 的422预览不是同一权限事件。
