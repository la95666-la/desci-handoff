# V3 Owner 地址迁移兼容

本修改移除 approved-mode Backend/Job/Admin 中对部署时 Owner Safe 地址的写死限制。它不是合约升级，不执行 Owner 移交，不修改数据库，发布前生产仍保持旧行为。

## 新规则

- 以被核验区块的 V3 Proxy `owner()` 为准；不使用 `pendingOwner()` 授权，不从 Admin 请求或数据库接收替代 Owner。
- 保留独立配置的 `idoV3TokenAdmission.ownerSafeCodeHash`，拒绝零地址、EOA、错误 Safe 字节码以及与 Proxy/Implementation/Governor/Operator 冲突的身份。
- Governor、Operator、Proxy、Implementation、chainId 和项目审批身份约束不变。
- Safe Token 提案仍核验既有 2/3 策略、nonce、SafeTxHash、签名人、目标、calldata/value/operation 和原始回执；向 Safe 服务提交前再次核对实际 Owner。
- 历史 receipt/原始创建审计使用历史区块的 Owner；最新读使用当前 Owner。迁移后不会把旧 Owner 合法执行的历史行为一律拒绝，也不会把新 Safe 当作历史 Owner。
- Legacy 签名 manifest 模式继续绑定其签名的 Owner 身份；不能静默改写已签授权。生产 approved-mode 显式选择 FollowOwner。

## 发布和交接边界

需要 Backend、独立 Job、Admin 对应版本分别发布。Backend 包内的 Job 二进制不会自动更新独立 jc-job。未部署新版前，不依据这份源码声称线上已支持新 Safe。

新 Safe 必须满足当前独立代码信任根；若使用另一 Safe 字节码/版本，仍需单独审阅并更新相应信任配置。移除的是固定地址，不是所有 Safe 校验，也不宣称支持任意治理合约或任意 threshold。

正常 Owner 两步交接仍是原 Safe `transferOwnership(newSafe)`，新 Safe `acceptOwnership()`；应在支持版本发布并验证后操作。交接不改变 Governor/Operator/Fee Receiver，不能使用 relayer 私钥绕过 Safe 或 outbox。

如同一历史区块内发生 Owner 交接及旧 Owner Token 配置，按区块末 Owner 绑定的 receipt reader 可能保守拒绝，需要独立逐交易核验，不能伪造成功。
