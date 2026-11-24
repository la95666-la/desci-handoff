# Fair Launch BSC 主网测试治理实录

> 状态：**测试用途，非生产治理准入**
>
> 网络：BNB Smart Chain mainnet，`chainId=56`
> 最近 Project Proceeds Safe 只读核验：tip `122158375`

## 1. 范围与限制

本记录只覆盖 Fair Launch 小额主网人工测试所需的 Safe/Timelock 治理地址。

- 所有链上写操作均由用户在钱包中逐笔确认。
- 不读取、记录或保管私钥、助记词、密码、原始签名或 raw transaction。
- 历史 IDO deployer `0xc7b4bf323C94CEb2E6e9bb7fbf89D06140941e5f`
  经用户明确批准，仅作为本轮测试 Safe Owner A 使用。
- 该例外不构成生产治理准入；生产前必须重新建立和核验独立签名域。
- 当前 Safe 不得安装 module/guard，不得修改 owners/threshold，不得在角色冻结前接收业务资产。
- 当前没有部署 Timelock、Fair Launch 合约或其他新治理合约。

## 2. Safe 公共依赖

| 组件 | BSC 主网地址 | 版本/用途 |
|---|---|---|
| SafeProxyFactory | `0x4e1DCf7AD4e460CfD30791CCC4F9c8a4f820ec67` | 官方 Safe v1.4.1 factory |
| Safe singleton | `0x41675C099F32341bf84BFc5382aF534df5C7461a` | 创建时初始 singleton |
| SafeL2 singleton | `0x29fcB43b46531BcA003ddC8FCB67FFE91900C762` | BSC 最终 singleton，`1.4.1+L2` |
| SafeToL2Setup | `0xBD89A1CE4DDe368FFAB0eC35506eEcE0b1fFdc54` | 创建时切换至 SafeL2 |
| CompatibilityFallbackHandler | `0xfd0732Dc9E303f09fCEf3a7388Ad10A83459Ec99` | 官方 fallback handler |

## 3. 测试 Owner 集

所有已创建 Safe 当前使用相同的 2/3 Owner 集：

| Owner | 地址 | 说明 |
|---|---|---|
| A | `0xc7b4bf323C94CEb2E6e9bb7fbf89D06140941e5f` | 历史 IDO deployer；用户批准的测试例外 |
| B | `0x35D3F22ab317569EA15728e59bC0EEeff521b687` | 新测试 Owner |
| C | `0x0C47E90BA6225c53a17650272f7bF4dC3dE805B3` | 新测试 Owner |

共同状态：

```text
threshold = 2
owner count = 3
Safe nonce = 0
modules = []
guard = 0x0000000000000000000000000000000000000000
module guard = 0x0000000000000000000000000000000000000000
balance = 0
```

## 4. 已创建并核验的 Safe

### 4.1 Governance test Safe

```text
address        0x79c527B8Af5b1E27d5052f8dCe04D20733e72350
creation tx    0x9fb768583346653dfa910f9f528512eaeebb7a67c72491a7439706c31d8dd1a9
creation block 122150762
confirmations  4829 at snapshot tip
status         1
```

预定测试用途：

```text
Governance Timelock proposer/executor
```

该 Safe 不直接成为 Fair Launch Owner；已部署的 Governance Timelock 才是 Owner 和 upgrade authority。

### 4.2 Emergency test Safe

```text
address        0x070EBAE10689CB90EEC1377d2F23b3825Ab8c0C8
creation tx    0xe6a279ff1c0b180b1ccd60421ea27ba8ce4ec2ec3a78a7a19d65dfb5e04be4b3
creation block 122151949
confirmations  3642 at snapshot tip
status         1
```

预定测试用途：

```text
Fair Launch Pause Guardian
Unpause Timelock proposer/executor
```

### 4.3 Treasury test Safe

```text
address        0x67D704A3C4c52f590C02050ee1e18652A6D784f1
creation tx    0x03a8c5391fd71e15077259f75afa6efeab2ed0dc7e997b79cbf5ab25639bd3d6
creation block 122152597
confirmations  2994 at snapshot tip
status         1
```

预定测试用途：

```text
Fair Launch initialTreasury
```

### 4.4 Platform Fee test Safe

```text
address        0xdBD2A66D827AeaFca9b65a6DC79501263730cbE1
creation tx    0x41f015d18bd2e4dc2eaf665c8b6f7224171fced77740c81b6289896e97918a77
creation block 122154667
confirmations  924 at snapshot tip
status         1
```

预定测试用途：

```text
Fair Launch initialPlatformFeeReceiver
withdrawPlatformFee beneficiary
```

### 4.5 Project Proceeds test Safe

```text
address        0x4aBA60fe416f5500E58eb07B5b487dbdF3A34dD9
creation tx    0xf389027b7fa80d96ee095be49edb562e86072731d300d9001528dbc329ca365f
creation block 122155332
confirmations  3044 at verification tip
status         1
```

预定测试用途：

```text
projectProceedsReceiver
withdrawProjectProceeds beneficiary
```

## 5. Timelock authority

### 5.1 Timelock

五个 Safe 和两个 OpenZeppelin `TimelockController` 均已完成部署与 finalized verification：

```text
Governance Timelock
  minDelay  = 172800 seconds (48h)
  proposer  = Governance test Safe
  executor  = Governance test Safe
  admin     = zero address

Unpause Timelock
  minDelay  = 3600 seconds (1h)
  proposer  = Emergency test Safe
  executor  = Emergency test Safe
  admin     = zero address
```
离线 deployment bundle 已在 Contract commit `fe29c78` 上生成：

```text
observation block  122160001
deployer           0xc7b4bf323C94CEb2E6e9bb7fbf89D06140941e5f
starting nonce     24
final nonce        26 (exclusive)
bundle digest      0x8485a37c4232de5a8e6b1d58a86a7d3d9d2f5ebc780d601c9ed907fed35dae04
bundle SHA-256     fedf4b2ab36b1e8d6f8b3ec2edfd9cd5df01ba15c33af80fc7e05886a02ca1dd
status              UNSIGNED_OFFLINE_REVIEW_REQUIRED
broadcastPermitted false
```

预测地址和 CREATE identity：

```text
Governance Timelock
  nonce       24
  address     0x0eE7f9f79B44446eFE8741037D5d46419266083D
  delay       172800
  data hash   0x46fdd97017b840b2192cba482b3ce78b3192d3264b866a519cfc4d1be818302c

Unpause Timelock
  nonce       25
  address     0xD8950F857a3485501E6cDcBE12Deae23911F35c0
  delay       3600
  data hash   0x91e2ec12efc6955385a081b2109a69bca3d817fc9a88bb78a37d82d4c430b239
```

共同构建 identity：

```text
Solidity              0.8.22+commit.4fc1097e
optimizer             enabled, 200 runs
viaIR                 true
OpenZeppelin Contracts 5.1.0
runtime code hash     0xeff9ee1c12de25a64930b1c2cf33522a9200069eb64a618db5c12e693959c63b
admin                 zero address
executor              closed to the corresponding Safe
```

Governance Timelock 已部署并完成 finalized read-only verification：

```text
address          0x0eE7f9f79B44446eFE8741037D5d46419266083D
transaction      0xb578086bfa055c1dc004e3ef68bd4b6ef597f850533b0ee0a86157d8c368aa0a
block            122168257
status           1
nonce            24
value            0 BNB
gas limit        1664817
gas used         1375265
finality         212 confirmations at verification tip
minDelay         172800 seconds
balance          0 BNB
```

验证结论：

- CREATE input 与离线 bundle 逐字节一致，Keccak-256 为 `0x46fdd97017b840b2192cba482b3ce78b3192d3264b866a519cfc4d1be818302c`。
- 实际部署地址与 nonce-24 预测地址一致。
- runtime code hash 精确等于 `0xeff9ee1c12de25a64930b1c2cf33522a9200069eb64a618db5c12e693959c63b`。
- Timelock 自身拥有 `DEFAULT_ADMIN_ROLE`；零地址、deployer 和 Governance Safe 均没有该角色。
- Governance Safe 独占 `PROPOSER_ROLE`、`CANCELLER_ROLE` 和 `EXECUTOR_ROLE`。
- deployer 不拥有 proposer、canceller、executor 或 admin 角色。
- 零地址不拥有 `EXECUTOR_ROLE`，执行权未公开。
- 构造 receipt 的 `RoleGranted` 与 `MinDelayChange(0, 172800)` 事件匹配。

Unpause Timelock 已部署并完成 finalized read-only verification：

```text
address          0xD8950F857a3485501E6cDcBE12Deae23911F35c0
transaction      0x441271d1ac21de5ed63329356895ef15b7282acd57c53dceeb850e73cba982ca
block            122170272
status           1
nonce            25
value            0 BNB
gas limit        1664817
gas used         1375265
finality         145 confirmations at verification tip
minDelay         3600 seconds
balance          0 BNB
deployer nonce   26 latest / 26 pending after deployment
```

验证结论：

- CREATE input 与离线 bundle 逐字节一致，Keccak-256 为 `0x91e2ec12efc6955385a081b2109a69bca3d817fc9a88bb78a37d82d4c430b239`。
- 实际部署地址与 nonce-25 预测地址一致。
- runtime code hash 精确等于 `0xeff9ee1c12de25a64930b1c2cf33522a9200069eb64a618db5c12e693959c63b`。
- Timelock 自身拥有 `DEFAULT_ADMIN_ROLE`；零地址、deployer 和 Emergency Safe 均没有该角色。
- Emergency Safe 独占 `PROPOSER_ROLE`、`CANCELLER_ROLE` 和 `EXECUTOR_ROLE`。
- deployer 不拥有 proposer、canceller、executor 或 admin 角色。
- 零地址不拥有 `EXECUTOR_ROLE`，执行权未公开。
- 构造 receipt 的 `RoleGranted` 与 `MinDelayChange(0, 3600)` 事件匹配。

两个 Timelock 的 self-admin、proposer、canceller、executor、delay、runtime code hash、receipt 和 20-block finality 均已核验。

### 5.2 已冻结的 Operator 和平台费率

用户已明确批准以下 BSC 主网小额测试参数：

```text
Operator          0x1cB633B1B9D8ABa8DD26acB36cA5780373048b9d
authority kind    durable-signer
code type         EOA
observed balance  0 BNB
observed nonce    0 latest / 0 pending
role collision    none
platform fee      250 bps / 2.5%
```

Operator 仅承担受限的日常链上执行职责：创建项目、激活项目、结束前取消项目，以及在满足 20-block finality 和 release evidence 约束后推进成功项目释放。它不是资金受益人，也不能升级合约、修改治理配置、暂停或解除暂停、提取资金、配置 token policy 或 recover excess tokens。

Operator 当前没有 BNB；启用 Backend/Job durable signer 前只能补充受控 gas 预算，不得向该地址转入项目款、平台费或 Treasury 资产。私钥、助记词和密码不得进入部署记录、源码、日志或对话。

### 5.3 其他未完成项

- Fair Launch 最终 deployment manifest、实际 deployer nonce、九笔 CREATE receipt 和 20-block finality。

## 6. 当前角色草案

| Fair Launch 角色 | 当前地址/状态 |
|---|---|
| Governance Owner / Upgrade Authority | `0x0eE7f9f79B44446eFE8741037D5d46419266083D` |
| Operator | `0x1cB633B1B9D8ABa8DD26acB36cA5780373048b9d` |
| Pause Guardian | `0x070EBAE10689CB90EEC1377d2F23b3825Ab8c0C8` |
| Unpause Timelock | `0xD8950F857a3485501E6cDcBE12Deae23911F35c0` |
| Treasury | `0x67D704A3C4c52f590C02050ee1e18652A6D784f1` |
| Platform Fee Receiver | `0xdBD2A66D827AeaFca9b65a6DC79501263730cbE1` |
| Project Proceeds Receiver | `0x4aBA60fe416f5500E58eb07B5b487dbdF3A34dD9` |
| Platform Fee | `250 bps / 2.5%` |

## 7. Fair Launch final unsigned deployment bundle

统一 authority snapshot：

```text
observation block  122173011
block hash         0x64ddbeedfa00529188dc9a887a8d601a49aa5e609c38bca6b2beac779e7798b5
deployer           0xc7b4bf323C94CEb2E6e9bb7fbf89D06140941e5f
starting nonce     26
final nonce        35 (exclusive)
deployer balance   0.009403432267153786 BNB
```

最终离线 bundle：

```text
contract commit    fe29c78c6c281cfeaa6507e8659e1d0c4dd45e41
bundle digest      0xc52e25ed8b9fc15bcd9fbc857553f8457d1da5329703a5d1a38467cec711265b
bundle SHA-256     c9e7cad3d19d41d9500980a6d8349199b5f0ea1c93de573dcb039454bfbd2896
full set digest    0xd0b95526b585f941fd65047f11e72fd3ca759caafe3398d356408b1dfd51d8f0
initializer hash   0x20d18a92f10bc967abc7bf20472719a888ec66658bdc56d28f8dd4bfc3a2888b
status             UNSIGNED_OFFLINE_REVIEW_REQUIRED
broadcastPermitted false
```

九笔 CREATE 顺序：

| Order | Nonce | Component | Predicted address | Gas cap |
|---:|---:|---|---|---:|
| 1 | 26 | Logic library | `0x7a78F769D0CE93615c9a1a78B7Df9cc9376fbd79` | 774634 |
| 2 | 27 | Runtime library | `0x3e08DB515592aCAe733119861999C179aC50ab6E` | 429161 |
| 3 | 28 | Core facet | `0x25c6808be02fEfe43D7574f8b34E731F844d7141` | 2794802 |
| 4 | 29 | Release facet | `0xF77af7c40788b9c6ecB070229b186bF6c95d099a` | 1284041 |
| 5 | 30 | Funds facet | `0x0902E0131DAC3C8Eb566997476f04db227AA5c2b` | 1769544 |
| 6 | 31 | Admin facet | `0xC7f62cccBFF48bF8920AfF57797d702eb6291D8E` | 2031802 |
| 7 | 32 | Views facet | `0x9D280BF09326546cf634AD14ee9723Cf503e918B` | 2155206 |
| 8 | 33 | Dispatcher implementation | `0x45BE772bF52624c3cA5938832571687c5C59384C` | 1349114 |
| 9 | 34 | ERC1967 proxy | `0xaEC57de2b134e3539edbB70a552483031a14BD84` | 368331 |

前八笔 gas cap 来自 BSC mainnet `eth_estimateGas` 的 120%；Proxy 依赖前八个预测地址，使用完整顺序 Hardhat receipt 的 120% cap。完整隔离链模拟总 gas 为 `10705327`，九个 runtime code hash、预测地址和 Proxy 初始化状态均与 bundle 精确一致。

Proxy initializer 冻结：

```text
governance owner     0x0eE7f9f79B44446eFE8741037D5d46419266083D
operator             0x1cB633B1B9D8ABa8DD26acB36cA5780373048b9d
pause guardian       0x070EBAE10689CB90EEC1377d2F23b3825Ab8c0C8
unpause timelock     0xD8950F857a3485501E6cDcBE12Deae23911F35c0
treasury             0x67D704A3C4c52f590C02050ee1e18652A6D784f1
platform fee receiver 0xdBD2A66D827AeaFca9b65a6DC79501263730cbE1
platform fee         250 bps / 2.5%
```

BSC USDT/USDC 不属于 initializer。部署完成后，必须由 Governance Timelock 通过独立受治理交易注册 policy revision 并启用 admission。

### 7.1 CREATE receipt progress

第 1/9 笔 Logic library 已完成 finalized verification：

```text
transaction       0x5eb339c5823c109eb284a2c70adc4829dbbe040386d597fc45bd41ab3dd579a3
block             122183790
status            1
confirmations     85 at verification tip
nonce             26
value             0 BNB
address           0x7a78F769D0CE93615c9a1a78B7Df9cc9376fbd79
gas limit         774634
gas used          639287
fee wei           55922828988125
input hash        0xdd4fd7d60e03cd3fed38202d89d7b0908b24ef7da36e0b31eefa9f69afa7cebe
runtime code hash 0x6492f8abb8e77c9d421923dbb036583d3122824243bbc6b992b89658806990d0
next nonce        27 latest / 27 pending
```

CREATE input、预测地址和 runtime code hash 均与 final bundle 精确一致。
第 2/9 笔 Runtime library 已完成 finalized verification：

```text
transaction       0x444e8fc200c4302cbf045d1432ff49bbe68e300967e7167de08474fbc834a876
block             122187421
status            1
confirmations     99 at verification tip
nonce             27
value             0 BNB
address           0x3e08DB515592aCAe733119861999C179aC50ab6E
gas limit         429161
gas used          353660
fee wei           17683000000000
input hash        0x74be23618c9a63a2bceab49b1b17af884d86bbf1f8ec9114dfc91ec733cf66ec
runtime code hash 0xd508f1cba6d943971ad6fd3f28bddc1979a4643ea9ff37026cf5fdbc1ae9b9b7
next nonce        28 latest / 28 pending
```

CREATE input、预测地址和 runtime code hash 均与 final bundle 精确一致。

第 3/9 笔 Core facet 已完成 finalized verification：

```text
transaction       0x1670eac0aa322a67399bdde5ad9b317f8438194bba7e985308986ccfec90c2bb
block             122188442
status            1
confirmations     99 at verification tip
nonce             28
value             0 BNB
address           0x25c6808be02fEfe43D7574f8b34E731F844d7141
gas limit         2794802
gas used          2309504
fee wei           170724891435008
input hash        0x66fcf765f6710c2691d3f92568d203d5267a116cd157f94c995eb0a2d6437a25
runtime code hash 0xecfab74e694ab890edf315d2d5b4531408f316f6ff9a2fd7f2cc09202d21610a
next nonce        29 latest / 29 pending
```

CREATE input、预测地址和 runtime code hash 均与 final bundle 精确一致。

第 4/9 笔 Release facet 已完成 finalized verification：

```text
transaction       0x8ec53ec502a81e027a573099fb53da6be8412de6efe9791d7bcd3251e3f8fe2f
block             122188967
status            1
confirmations     377 at verification tip
nonce             29
value             0 BNB
address           0xF77af7c40788b9c6ecB070229b186bF6c95d099a
gas limit         1284041
gas used          1060450
fee wei           77108837012650
input hash        0x5644980f5317ff93537f83e6bc7fe831f45a6614af4b846afd24a27223891562
runtime code hash 0x1c7e3c7c9da37f9552162f93c6e2e2751dcba1d37df5f0900928c5cc174cf1b4
next nonce        30 latest / 30 pending
```

CREATE input、预测地址和 runtime code hash 均与 final bundle 精确一致。

第 5/9 笔 Funds facet 已完成 finalized verification：

```text
transaction       0x6fbc218cf1ab761a6d8fd80a8f573e862d36eba8d29e67a5f9548e46b2edcae9
block             122190618
status            1
confirmations     165 at verification tip
nonce             30
value             0 BNB
address           0x0902E0131DAC3C8Eb566997476f04db227AA5c2b
gas limit         1769544
gas used          1461851
fee wei           85902114407815
input hash        0x0f4e7de9e0b89a401fe4f6f9d2dfc09ac26c58578f90870c09cbfe57bc53fde7
runtime code hash 0xdb90423c0eb894e951e512a3412f0cc736676b928e89444ea255eb926fbc307d
next nonce        31 latest / 31 pending
```

CREATE input、预测地址和 runtime code hash 均与 final bundle 精确一致。

第 6/9 笔 Admin facet 已完成 finalized verification：

```text
transaction       0x041e68f0093cd26640d6cec55bc06a6c4a9081f2fb86c430777d63b15fdeed77
block             122191838
status            1
confirmations     133 at verification tip
nonce             31
value             0 BNB
address           0xC7f62cccBFF48bF8920AfF57797d702eb6291D8E
gas limit         2031802
gas used          1678678
fee wei           120193344800000
input hash        0x71e4be321ccac75e354a6add505ddef83298ba8662b03c2894972a8c7b2096be
runtime code hash 0x467bd3a49dbc40a5f8777be4c3a38113329d588806dff1da3ec282c171887154
next nonce        32 latest / 32 pending
```

CREATE input、预测地址和 runtime code hash 均与 final bundle 精确一致。

第 7/9 笔 Views facet 已完成 finalized verification：

```text
transaction       0xaffbba77a4280db52efc4a999f8791c244d2c9e5e3a6f3d9df6e9cdb81ba66ef
block             122192359
status            1
confirmations     107 at verification tip
nonce             32
value             0 BNB
address           0x9D280BF09326546cf634AD14ee9723Cf503e918B
gas limit         2155206
gas used          1780705
fee wei           132395849461315
input hash        0x14c106dd3225c3ab7a18baa38f56388a9ee6bbde632539b34b156e621bd4099f
runtime code hash 0x4e2480b3d5c3a455e4b6f4b60222dfcf74d728c9395ce10cc07a58279bf86f0a
next nonce        33 latest / 33 pending
```

CREATE input、预测地址和 runtime code hash 均与 final bundle 精确一致。

第 8/9 笔 Dispatcher implementation 已完成 finalized verification：

```text
transaction       0x0f196ab0400579f1e2e12b95f0ee77332b5f8229344a3d1d9827012e7193073a
block             122192833
status            1
confirmations     99 at verification tip
nonce             33
value             0 BNB
address           0x45BE772bF52624c3cA5938832571687c5C59384C
gas limit         1349114
gas used          1114250
fee wei           84089140406000
input hash        0x2d27954321e94bd78c22b3717ac02570a2553ceaff62c67eaf9ffa34a70196f1
runtime code hash 0x35b4f5b4e71d62242e67df5e68d9e075a3bcb4f64eb46314b6a6abcdd1a3322f
next nonce        34 latest / 34 pending
```

五个 immutable facet constructor arguments、CREATE input、预测地址和 runtime code hash 均与 final bundle 精确一致。

第 9/9 笔 ERC1967 Proxy 已完成 finalized verification 与部署后状态验收：

```text
transaction       0x0183670dddc0943cd7f58ef7b1724892460e9784088da5f22fabc10d19043559
block             122194506
block hash        0x1a60b4adbb026ce7f59280eaf6da13016851c577c123014d8fa96e209d343476
status            1
confirmations     579 at final verification tip
nonce             34
value             0 BNB
proxy             0xaEC57de2b134e3539edbB70a552483031a14BD84
gas limit         372654
gas used          306942
fee wei           16028511240000
input hash        0xaef7180c419fdf4dee56b014077496ea7171bab807bfeb941d074c6e60b9703a
proxy runtime hash 0x160941bd580b2632c64b1811d3438319bfce40e0418e8ad850fb541ee6ee74ee
implementation    0x45BE772bF52624c3cA5938832571687c5C59384C
next nonce        35 latest / 35 pending
```

CREATE input、预测地址、Proxy runtime hash 和 ERC-1967 implementation slot 均与 final bundle 精确一致。已逐区块验证部署区块 `122194506` 至 `122194526` 的 20 个 successor parent links 连续。

部署后状态：

```text
owner / upgrade authority  0x0eE7f9f79B44446eFE8741037D5d46419266083D
pending owner              0x0000000000000000000000000000000000000000
operator                   0x1cB633B1B9D8ABa8DD26acB36cA5780373048b9d
pause guardian             0x070EBAE10689CB90EEC1377d2F23b3825Ab8c0C8
unpause timelock           0xD8950F857a3485501E6cDcBE12Deae23911F35c0
treasury                   0x67D704A3C4c52f590C02050ee1e18652A6D784f1
platform fee receiver      0xdBD2A66D827AeaFca9b65a6DC79501263730cbE1
platform fee               250 bps / 2.5%
contract version           keccak256("fair_launch_v1")
claims paused              false
USDT admission             unconfigured / disabled
USDC admission             unconfigured / disabled
```

Receipt events 已解码为：

```text
Upgraded             0x45BE772bF52624c3cA5938832571687c5C59384C
OwnershipTransferred zero -> Governance Timelock
Initialized           version 1
```

全部 Logic、Runtime、Core、Release、Funds、Admin、Views、Dispatcher 与 Proxy runtime hashes 均与 full-set manifest 匹配。Governance Timelock、Unpause Timelock、Operator、Treasury、Platform Fee Safe 和 Project Proceeds Safe 的 code identity 未漂移；timelock delay 仍为 48h / 1h。

只读负向验证确认：重复初始化、implementation 初始化、deployer 修改费率、deployer 修改 Operator、deployer 升级均回滚；renounce ownership 永久回滚。deployer 不具备 Fair Launch 管理权限。


## 8. 下一停点

1. 九笔 CREATE 部署序列已完成；不得再次使用 deployment bundle 广播任何交易。
2. 由 Governance Timelock 准备并执行 USDT/USDC payment-token policy 与 admission 操作。
3. 在测试服务器 activation、scanner、materializer、Frontend wallet lane 和 Admin dispatch 全部 admission-protected 前，继续保持业务交易与 release fail closed。
4. 完成测试服务器只读盘点、clean release bundle、备份与 rollback artifact 后，才进入最小真实资金生命周期测试。
