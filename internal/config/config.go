package config

// EnableFileLog controls file logging to overlay.log.
// Set to true to enable, false to disable.
var EnableFileLog = false

// UploadEndpoint 战斗历史上传服务器地址，通过 .env、环境变量或 CI 注入。
var UploadEndpoint = ""

// UploadEnabled 是否启用战斗数据推送。
var UploadEnabled = true

// UploadDungeonKeyword 仅上传文件名/副本名包含任一关键字的战斗记录（逗号分隔）。
var UploadDungeonKeyword = "布里列赫,佩斯皮亚德"

// MinUploadTargetMaxHP 上传时仅保留 Boss 最大血量不低于该值的目标（单位：点）。
const MinUploadTargetMaxHP = 200_000_000

// MinUploadTargetMaxHPPeisiaad 佩斯皮亚德上传阈值（简单模式主塔约 262,500）。
const MinUploadTargetMaxHPPeisiaad = 200_000

// ClientVersion 客户端版本号，上传时附带。
const ClientVersion = "2.2.2"

// UploadSecret 上传 HMAC 签名密钥，从 .env 或环境变量 BLONY_UPLOAD_SECRET 加载。
// 留空时不发起上传。
var UploadSecret = ""

// UploadSecretPlaceholder 历史占位符，若误写入本地配置则视为未配置。
const UploadSecretPlaceholder = "CHANGE_ME_BEFORE_RELEASE"

// UploadSignatureMaxSkewSeconds 签名时间戳允许偏差（秒），服务端验签时应使用相同窗口。
const UploadSignatureMaxSkewSeconds = 300
