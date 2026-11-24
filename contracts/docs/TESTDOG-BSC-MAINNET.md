# TESTDOG — BSC 主网测试项目币

| 项目 | 值 |
|---|---|
| 网络 | BNB Smart Chain 主网，chain ID `56` |
| 合约地址 | `0x5aeeC1e427586922D676f08B453ce692B13C7eD4` |
| 名称 / 符号 | `TESTDOG` / `TESTDOG` |
| 精度 | `18` |
| 初始铸造量 | `1,000,000 TESTDOG`，铸给初始 Owner |
| 最大发行量 | `10,000,000 TESTDOG`（`maxSupply` 硬顶） |
| 部署及初始 Owner | `0x6C099660A0EE289dE73466Fe24F88FbfA81d3a93` |
| 部署交易 | `0x1a02e83173483e9bf73f6d9450d24111197c5414a907bebabb1f9d6f97962266` |
| 部署区块 | `129729409`；原交易回执 `status=1` |
| 合约源码 | `contracts/mocks/TestProjectToken.sol` |

合约基于 ERC-20 与 `Ownable2Step`，持有人可 `burn`，Owner 可在硬顶内 `mint`。这是一枚**已部署到真实主网**的测试代币，不是 USDT、项目的投资凭证或白名单 proof。主网交易需要真实 BNB 和支付币。

## V3 项目币准入

Owner Safe `0x79c527B8Af5b1E27d5052f8dCe04D20733e72350` 已对独立 V3 Proxy `0x0D4040875742975269c55E16C8884d3Ee9510B8E` 执行 `configureProjectToken(TESTDOG, true)`：

- 原始链上交易：`0x59e1b4ef65709fc014f79d98c4bae2dfee845bdd91fb7c793a40eb7453f6b0d2`。
- 交易成功，项目币配置读回为 `allowed=true`、`decimals=18`。**不要重复发起准入交易**；状态变化应重新从链上核验。

项目币准入不等于已为某 IDO 项目创建 Set、上链、向项目充值，也不赋予任意钱包白名单投资资格。项目的资格仍由独立 Policy Safe 策略、Job 规范事件与单钱包 proof 决定。原交易结果未知时只按原哈希查询，不盲目重发。

只读链上链接：<https://bscscan.com/address/0x5aeeC1e427586922D676f08B453ce692B13C7eD4>。
