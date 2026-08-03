# DPS Pusher 上传接口对接文档

BlonyMonitorV2 客户端在本地保存战斗历史后，会按条件自动将数据 POST 到配置的上传地址。

## 基本信息

| 项目 | 值 |
|------|-----|
| Endpoint | 由 `.env`、环境变量或 GitHub **Secrets** 配置 |
| HTTP 方法 | `POST` |
| Content-Type | `multipart/form-data` |
| 鉴权 | `HMAC-SHA256` 请求签名（必填） |
| 超时 | 客户端 15 秒 |

客户端配置分三层（优先级从低到高）：

1. `internal/config/config.go` — 代码中的公开默认值（可入库）
2. `.env` — 运行时本地文件（**不入库**，见 `.gitignore`）
3. 系统环境变量 — 最高优先级

**官方 Release 由 GitHub Actions 在编译时通过 `-ldflags` 将 Secrets / Variables 注入 exe，Release zip 中不包含 `.env`。**

| 配置项 | 说明 |
|--------|------|
| `UploadSecret` | HMAC 密钥（敏感，走 GitHub **Secret**） |
| `UploadEndpoint` | 上传地址（敏感，推荐走 GitHub **Secret**） |
| `RankingConsentEndpoint` | 排行参与状态地址（敏感，走 GitHub **Secret**） |
| `AnnouncementEndpoint` | 服务端公告地址（敏感，走 GitHub **Secret**） |
| `UploadDungeonKeyword` | 副本名关键字过滤 |
| `MinUploadTargetMaxHP` | 最低 Boss 血量过滤（仅代码配置） |

### GitHub Secrets 与 Variables（维护者推荐）

路径：**仓库 Settings → Secrets and variables → Actions**

#### Secrets（加密，日志中自动打码）

| 名称 | 必填 | 说明 |
|------|------|------|
| `BLONY_UPLOAD_SECRET` | 是 | HMAC 签名密钥，与服务端一致，建议 ≥32 位随机字符串 |
| `BLONY_UPLOAD_ENDPOINT` | 是 | 推送地址（推荐放 Secrets，避免在仓库 Variables 中明文可见） |
| `BLONY_RANKING_CONSENT_ENDPOINT` | 是 | 排行参与状态地址，只放 Secrets，不在仓库中公开路径 |
| `BLONY_ANNOUNCEMENT_ENDPOINT` | 是 | 服务端公告地址，只放 Secrets，不在仓库中公开路径 |

添加步骤：

1. 打开 GitHub 仓库 → **Settings**
2. 左侧 **Secrets and variables** → **Actions**
3. 切到 **Secrets** 标签 → **New repository secret**
4. 分别添加 `BLONY_UPLOAD_SECRET`、`BLONY_UPLOAD_ENDPOINT`、`BLONY_RANKING_CONSENT_ENDPOINT` 与 `BLONY_ANNOUNCEMENT_ENDPOINT`

#### Variables（非敏感，可公开在仓库内）

| 名称 | 必填 | 说明 |
|------|------|------|
| `BLONY_UPLOAD_DUNGEON_KEYWORD` | 否 | 副本关键字，默认 `布里列赫` |
| `BLONY_UPLOAD_ENABLED` | 否 | `true` / `false`，默认 `true` |

> 若将 `BLONY_UPLOAD_ENDPOINT` 放在 **Variables** 而非 Secrets，CI 也会识别（作为 Secrets 未配置时的回退）。

添加步骤：

1. 同上进入 **Secrets and variables → Actions**
2. 切到 **Variables** 标签 → **New repository variable**
3. 按需添加 `BLONY_UPLOAD_DUNGEON_KEYWORD` 等

#### CI 如何注入

`.github/workflows/release.yml` 在 **Build application** 步骤读取 Secrets / Variables，通过 Go `-ldflags` 编译进 exe（密钥以 Base64 注入，避免明文出现在构建参数中）：

```
zip 内容示例：
  BlonyMonitorV2.exe   ← 密钥已编译进二进制
  mabidata.db
  sounds/
```

推送 `main` 或打 `v*` 标签触发构建后，下载 Release 即可使用，**密钥不会出现在 GitHub 源码或 Release 附件的明文配置文件中**。

### 本地开发 / Fork 自编译

没有仓库 Secrets 权限时，复制 **`.env.example`** 为 **`.env`**（项目根目录或 `internal/config/.env`，已 gitignore）：

```env
BLONY_UPLOAD_SECRET=你的密钥
BLONY_UPLOAD_ENDPOINT=你的推送地址
BLONY_RANKING_CONSENT_ENDPOINT=你的排行参与状态地址
BLONY_ANNOUNCEMENT_ENDPOINT=你的公告地址
```

加载优先级：

```
config.go 默认值 < CI ldflags 注入 < .env < 系统环境变量
```

> **注意**：密钥和服务地址编译进 exe 后，熟练用户仍可能通过逆向提取。相比附带 `.env` 文件，这种方式不会让用户解压即见明文配置，但无法做到绝对保密。若需更强防护，应改为用户注册 + 每用户独立 Token。

客户端不会通过 Wails 接口、调试面板或日志返回服务地址；错误信息中的地址统一显示为 `[server]`。

## 上传触发条件

同时满足以下条件时才会上传：

1. `UploadEndpoint` 非空
2. `UploadSecret` 已配置且不是占位符
3. 保存时的副本/场景名包含 `布里列赫`（`UploadDungeonKeyword`）
4. 已识别到玩家自身 ID（`selfId`）
5. 过滤后至少保留 1 个目标

上传在**后台异步**执行，失败时**不会弹窗或通知用户**，仅写入日志（需开启 `EnableFileLog` 才会落盘）。

---

## 鉴权：HMAC 签名

### 请求头

| Header | 必填 | 说明 |
|--------|------|------|
| `Authorization` | 是 | 格式：`HMAC-SHA256 <hex_signature>` |
| `X-Timestamp` | 是 | Unix 秒级时间戳（字符串） |
| `X-Nonce` | 是 | 16 字节随机数的 hex（32 字符），每次请求唯一 |

### 签名算法

```
fileHash = hex(SHA256(file 二进制内容))
payload  = timestamp + "\n" + nonce + "\n" + playerId + "\n" + fileHash
signature = hex(HMAC-SHA256(secret, payload))
```

说明：

- `timestamp`、`nonce`、`playerId` 均来自请求（`playerId` 为 multipart 字段）
- `file` 为 gzip 压缩后的**原始字节**，与服务端收到的文件内容一致
- 时间戳允许偏差：**±300 秒**（`UploadSignatureMaxSkewSeconds`）

### 服务端验签流程（必须按此顺序）

```
1. 读取 Authorization / X-Timestamp / X-Nonce
2. 校验时间戳在 ±300 秒内
3. 检查 nonce 未使用过（Redis SET NX，TTL 600s），否则 401
4. 解析 multipart，读取 playerId 与 file
5. 计算 fileHash，按公式重算 signature
6. 使用 hmac.Equal 与 Authorization 中的签名比较
7. 验签通过后再做限流、去重、语义校验
```

### Go 验签示例

```go
func verifyUpload(secret string, timestamp int64, nonce, playerID string, gzData []byte, authHeader string) bool {
    const prefix = "HMAC-SHA256 "
    if !strings.HasPrefix(authHeader, prefix) {
        return false
    }
    provided := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))

    fileHash := sha256.Sum256(gzData)
    payload := fmt.Sprintf("%d\n%s\n%s\n%x", timestamp, nonce, playerID, fileHash)
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(payload))
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(provided))
}
```

---

## 请求格式

### multipart 字段

| 字段名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `playerId` | string | 是 | 上传者游戏内实体 ID |
| `playerName` | string | 否 | 上传者角色名 |
| `dungeonName` | string | 是 | 副本/场景名 |
| `fileName` | string | 是 | 本地文件名 |
| `clientVersion` | string | 是 | 客户端版本 |
| `contentSha256` | string | 是 | `file` 的 SHA-256 hex，便于服务端快速校验 |
| `file` | file | 是 | gzip 压缩的 JSON 战斗数据 |

### 示例（curl，需自行计算签名）

```bash
# 实际请求由客户端自动签名；手动测试时需按上述算法生成 Authorization
curl -X POST "https://your-server.example/dpsPusher" \
  -H "Authorization: HMAC-SHA256 <signature>" \
  -H "X-Timestamp: 1739260800" \
  -H "X-Nonce: a1b2c3d4e5f6789012345678abcdef01" \
  -F "playerId=123456789" \
  -F "playerName=测试角色" \
  -F "dungeonName=布里列赫" \
  -F "fileName=2026-07-12_15-04-05_布里列赫.json.gz" \
  -F "clientVersion=2.2.2" \
  -F "contentSha256=<sha256-hex>" \
  -F "file=@battle.json.gz;type=application/gzip"
```

---

## 公开排行参与状态

“启用推送”和“参与公开排行”是两个不同的设置。排行有三种状态：

| 选择 | 公开榜单表现 |
|------|--------------|
| `none` 不参与排行 | 不进入任何公开排行 |
| `anonymous` 匿名排行 | 参与统计，但不展示角色名；服务端应使用不可逆的匿名标识 |
| `public` 公开排行 | 参与统计并展示角色名 |

本机推送开关只控制当前客户端是否发送战斗文件；队友上传同场数据时，服务端仍必须按每个角色自己的排行状态过滤。

排行状态按 `playerId`（角色）保存，不按电脑保存。切换角色后客户端会重新查询。关闭排行不能阻止队友客户端发送整场战斗文件，但服务端必须在入榜和查询时排除该角色。

### 隐私默认值与历史数据

- 服务端对不存在状态记录的 `playerId` 必须按 `mode=none` 处理，即明确选择后才进入排行。
- `PUT mode=none` 成功后，服务端必须立即让该角色从所有公开榜单消失，包括以前已产生的排行记录。
- 推荐在查询排行时实时关联当前同意状态，或在更新状态的同一事务中修改历史排行可见性。不能只在下一次上传时生效。
- 上传者的同意状态不能代替队友的状态；服务端必须逐个检查 `targets[].attackers[]` 中 `isPC=true` 的角色 ID。
- 原始战斗归档如需长期保留，应另设保留期和访问控制。退出排行不等同于删除原始战斗文件。

### 排行状态接口

Endpoint 由 `BLONY_RANKING_CONSENT_ENDPOINT` 注入，同一路径支持以下方法：

| 方法 | 请求体 | 语义 |
|------|--------|------|
| `GET` | 无 | 查询 `X-Player-ID` 对应状态；不存在时返回 `none` |
| `PUT` | JSON | 覆盖保存当前角色状态，操作必须幂等 |

共同请求头：

| Header | 说明 |
|--------|------|
| `Authorization` | `HMAC-SHA256 <hex_signature>` |
| `X-Timestamp` | Unix 秒级时间戳 |
| `X-Nonce` | 每次请求唯一的 16 字节随机数 hex |
| `X-Player-ID` | 当前客户端识别到的自身角色 ID |
| `X-Player-Name` | 当前角色名，可选，仅用于显示/审计 |

`PUT` 请求体：

```json
{
  "mode": "anonymous",
  "playerName": "测试角色",
  "clientVersion": "2.2.2"
}
```

`GET` 与 `PUT` 均返回：

```json
{
  "mode": "none",
  "updatedAt": "2026-07-21T12:00:00Z"
}
```

签名原文：

```text
bodyHash = hex(SHA256(原始请求体字节))
payload  = timestamp + "\n" + nonce + "\n" + UPPER(method) + "\n" + playerId + "\n" + bodyHash
signature = hex(HMAC-SHA256(secret, payload))
```

服务端必须校验时间窗口、nonce 防重放、请求头中的 `playerId` 与签名原文一致，并对 `PUT` 做审计与限流。当前共享 HMAC 只能防止普通伪造，不能严格证明游戏角色所有权；若面向对抗环境，应升级为账号登录后签发的每用户 Token 或游戏内一次性验证流程。

### 服务端入榜流程

```text
接收并验证战斗上传
  -> 解压并校验战斗数据
  -> 收集所有 isPC=true 的 attacker.id
  -> 批量查询 ranking_consent（缺失视为 none）
  -> none 不入榜，anonymous 匿名入榜，public 公开入榜
  -> 原始文件按独立的保留策略处理
```

建议状态表至少包含 `player_id`（唯一键）、`mode`、`updated_at`、`player_name`、`client_version` 与审计字段。任何排行缓存也必须在切换状态时失效。

## 服务端公告接口

Endpoint 由 `BLONY_ANNOUNCEMENT_ENDPOINT` 注入。客户端每次启动时发起带 HMAC 的 `GET`，服务端可以返回单个公告、公告数组，或 `{ "announcements": [...] }`。`timestamp` 使用 Unix 秒级整数，客户端只选择该值最大的公告。

公告格式：

```json
{
  "announcements": [
    {
      "timestamp": 1722500000,
      "title": "排行规则更新",
      "html": "<p>公告正文，可使用<strong>基础 HTML</strong>。</p>"
    }
  ]
}
```

公告正文会在客户端经过 HTML 白名单清洗，只允许常用排版、列表、表格、代码和 HTTP(S) 链接。客户端确认后保存最大时间戳；下次启动只展示 `timestamp` 大于本地已确认时间戳的公告。服务端删除旧公告不会让已确认公告重复出现。

公告请求使用与排行接口相同的时间戳、nonce 和 HMAC 机制，签名中的 `playerId` 固定为 `announcement`，请求体为空。服务端仍应限流，并返回 2xx JSON；没有新公告时返回空数组即可。

---

## 服务端防护清单

### 1. 限流（必做）

| 维度 | 建议阈值 |
|------|----------|
| 每 IP | ≤ 30 次 / 分钟 |
| 每 playerId | ≤ 20 次 / 小时 |
| 全局 | ≤ 500 次 / 分钟 |

超出返回 `429 Too Many Requests`。

### 2. 去重（必做）

以下任一组合命中则返回 `200`（幂等）或 `409`，**不要重复入库**：

- `playerId` + `fileName`
- `playerId` + `contentSha256`

建议对 nonce 使用 Redis：`SET upload:nonce:<nonce> 1 EX 600 NX`。

### 3. 文件校验

| 检查项 | 规则 |
|--------|------|
| 文件大小 | ≤ 10 MB |
| Content-Type | `application/gzip` 或 `application/octet-stream` |
| gzip 可解压 | 失败返回 400 |
| JSON 根字段 | 必须含 `targets` 数组且非空 |
| contentSha256 | 须与 `file` 实际 hash 一致 |

### 4. 语义校验（推荐）

| 检查项 | 规则 |
|--------|------|
| dungeonName | 包含 `布里列赫` |
| targets[].bossHP.maxHp | ≥ 200,000,000 |
| targets[].duration | 10 ~ 7200 秒 |
| targets[].totalDamage | > 0 |
| playerId | 纯数字字符串，长度合理（如 1~20 位） |
| cleanedAt / appearedAt | 非未来时间，不早于 30 天前 |

### 5. 传输安全

生产环境请将 Endpoint 改为 **HTTPS**，防止密钥与战斗数据被窃听。

---

## 文件内容（gzip 解压后 JSON）

```json
{
  "targets": [ /* targetExport 数组 */ ]
}
```

### 客户端上传前过滤规则

| 规则 | 说明 |
|------|------|
| 副本过滤 | 仅 `布里列赫` 副本触发上传 |
| 血量过滤 | 仅保留 `bossHP.maxHp >= 200000000` 的目标 |
| 宠物数据 | **保留**（移除会造成 totalDamage / percent 偏差） |

---

## 数据结构参考

### targets[] — 单个 Boss 目标

| 字段 | 类型 | 说明 |
|------|------|------|
| `targetId` | string | 目标实体 ID |
| `targetName` | string | 目标显示名 |
| `totalDamage` | number | 有效总伤害 |
| `dps` | number | 秒伤 |
| `duration` | number | 战斗时长（整数秒，至少 1 秒） |
| `cleanedAt` | int64 | 保存时间（厘秒） |
| `appearedAt` | int64 | 首次受击时间（厘秒） |
| `deathTime` | int64 | 死亡时间（厘秒，可选） |
| `attackers` | array | 攻击者列表 |
| `bossHP` | object | Boss 血量时间线 |

### attackers[] — 攻击者

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 攻击者实体 ID |
| `name` | string | 显示名 |
| `totalDamage` | number | 总伤害 |
| `dps` | number | 秒伤 |
| `percent` | number | 占目标总伤百分比 |
| `isPC` | boolean | 是否为玩家角色 |
| `lastHit` | int64 | 最后命中时间（厘秒） |
| `appearedAt` | int64 | 首次命中时间（厘秒） |
| `skills` | array | 技能聚合统计 |
| `skillsDetail` | array | 技能明细（含 `hitRecords`） |

### hitRecords[] — 逐击记录

| 字段 | 类型 | 说明 |
|------|------|------|
| `seq` | int64 | 伤害序号 |
| `damage` | number | 有效伤害 |
| `rawDamage` | number | 原始伤害 |
| `overflowDamage` | number | 溢出伤害 |
| `adjusted` | boolean | 是否经溢出修正 |
| `isCritical` | boolean | 是否暴击 |
| `timestamp` | int64 | 时间（厘秒） |

## 时间戳换算

```
毫秒 = timestamp * 10
秒   = timestamp / 100
```

## 建议的服务端处理流程

```
1. 读取并校验签名（Authorization / X-Timestamp / X-Nonce）
2. 限流检查（IP + playerId）
3. 解析 multipart，校验 contentSha256
4. gunzip(file) → JSON 解析
5. 语义校验（副本名、血量、时长等）
6. 去重（playerId + contentSha256）
7. 写入统计表，可选归档原始 gz
8. 返回 HTTP 2xx
```

## 响应建议

客户端**不解析响应体**，仅检查 HTTP 状态码是否在 2xx：

```json
{
  "ok": true,
  "reportId": "uuid-or-hash"
}
```

| 状态码 | 场景 |
|--------|------|
| 401 | 签名无效、时间戳过期、nonce 重放 |
| 400 | 文件/JSON/语义校验失败 |
| 409 | 重复上报（可选） |
| 429 | 限流 |
| 2xx | 成功 |

非 2xx 时客户端静默忽略，不提示用户。

## 版本记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-07-12 | 初始版本：multipart 上传 gzip 战斗快照 |
| 1.1 | 2026-07-12 | 增加 HMAC 签名、contentSha256、服务端防护清单 |
| 1.2 | 2026-07-15 | 存档字段与 EMA 对齐：攻击者时间、显式 isPC、整数时长与暴击区间语义 |
| 1.3 | 2026-07-21 | 增加按角色保存的公开排行参与状态、服务端过滤规则与私有 Endpoint 注入 |
| 1.4 | 2026-08-03 | 排行扩展为不参与/匿名/公开三态，增加按时间戳确认的 HTML 服务端公告 |
