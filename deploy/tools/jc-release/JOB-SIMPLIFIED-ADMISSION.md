# IDO Job 简化发布准入（候选；未部署生产）

按项目负责人要求，新 Job 发布不再强制要求制品/证据公钥、双签、六份回执或短时效 readiness 文件。已签名的旧格式发布仍按原规则校验，不将签名失败降级为无签名发布。

按既有流程由 CI/CD 生成 `joincare/job:<tag>` 的 gzip 压缩 `docker save` 组件包，操作员仅上传包、手动选择 Job 版本，并向应用机 `jc-agent` 下发升级命令；不改用 SSH 直接操作容器。Server 从包内 Docker `manifest.json` / 镜像 config 自动生成规范 `job_admission`，绑定上传归档 SHA-256 和 `sha256:` 镜像 ID，不要求操作员另交 JSON。新制品使用 `generation=0` 表示由 Agent 根据本机状态分配下一个 generation；部署时严格递增。该描述只绑定输入，**不代表 DB、RPC 或 signer 已被独立审查**。

Agent 仍在下载前及容器修改前核验描述，要求独立 Job 配置树、Job 私钥独立且无 Core 私钥、IDO-only 角色、Compose 只读隔离；归档下载时校验哈希，Docker load 后核对镜像 ID，临时 override 固定不可变镜像，检查生效 Compose；先记录 `job_pending`，仅修改 Job，健康检查失败按既有回滚规则恢复。已有 `job_pending` 必须先人工恢复。Fair Launch 财务物化继续禁用。

**剩余上线前人工核验：** 真实生产镜像及来源、数据库版本/恢复能力、Core signer 控制与 nonce/outbox、独立 Job signer、BSC proxy/implementation/operator、历史 RPC 能力、切换区块后的活动空档、V2 scanner 注册及健康检查与回滚。不得把旧审阅 ELF/占位镜像当作正式镜像。修改 `jc-release` 后须另做来源绑定构建、Server/Agent 回归、备份和分别升级；本文档不是生产启动回执。
