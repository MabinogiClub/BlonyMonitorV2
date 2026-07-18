# BlonyMonitorV2

洛奇（Mabinogi）实时 DPS 监控悬浮窗，从 BlonyMonitor 精简重构而来。

## 功能

- 实时抓包解析战斗伤害
- 造成伤害 / 受到伤害 / 玩家时间轴 / 角色列表
- DPS 趋势图表
- 频道与网卡选择
- 窗口透明、置顶、系统托盘

## 环境要求

- Go 1.24+
- Node.js 20.19+ 或 22.12+
- [Npcap](https://npcap.com/)（抓包）
- Wails v2 CLI

## 开发

```bash
cd new
wails dev
```

## 构建

```bash
cd new
wails build
```

## 项目结构

```
new/
├── main.go              # 入口
├── internal/
│   ├── app/             # 业务逻辑（抓包、DPS、窗口）
│   ├── packet/          # 协议解析
│   ├── pcaputil/        # 网卡探测
│   ├── constants/       # 频道/加速器配置
│   ├── config/          # 编译期配置
│   └── tray/            # 系统托盘
├── db/                  # SQLite 数据访问
└── frontend/            # Vue 3 前端
```
