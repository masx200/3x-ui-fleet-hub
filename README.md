[English](README.md) | [中文](README.zh_CN.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

# 3X-UI Fleet Hub

[![Release](https://img.shields.io/github/v/release/masx200/3x-ui-fleet-hub.svg)](https://github.com/masx200/3x-ui-fleet-hub/releases)
[![GO Version](https://img.shields.io/github/go-mod/go-version/masx200/3x-ui-fleet-hub.svg)](#)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)

**3X-UI Fleet Hub** — 基于 [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) 的增强版本，是一个先进、开源的基于Web的控制面板，用于管理 Xray-core 服务器。它提供用户友好的界面来配置和监控各种 VPN 和代理协议。

> [!IMPORTANT]
> 本项目仅供个人使用，请勿用于非法目的，请勿在生产环境中使用。

## 关于本项目

本项目是从 [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) 项目（基于2026年3月25日的版本）修改而来的增强版本。我们致力于改进前端架构、性能优化和功能扩展。

### 主要改进

相比上游项目，本版本自2026年3月25日以来进行了以下重大改进：

#### 1. 🚀 前端现代化改造
- **esbuild 构建系统**：引入 esbuild 替代传统构建方式，大幅提升构建速度
- **资源哈希化**：所有 JS/CSS 资源文件自动添加内容哈希，优化浏览器缓存策略
- **前端代码重构**：将所有前端资源从 `assets/` 移至 `src/` 目录，采用模块化组织结构
- **Vazirmatn 字体支持**：添加 Vazirmatn UI NL Regular 字体，改善多语言显示效果

#### 2. 🧪 出站节点批量测试功能
- **异步批量延迟测试**：实现一键测试所有出站节点的延迟功能
- **测试结果持久化**：新增 `OutboundTestResult` 数据库模型，保存每次测试结果
- **实时进度显示**：测试过程中实时显示进度条和当前节点状态
- **测试取消功能**：支持中途取消批量测试，立即停止轮询
- **测试统计**：显示成功/失败数量和总耗时

#### 3. ⚡ 性能优化
- **出站列表分页**：出站节点列表支持分页显示，提升大量节点时的性能
- **搜索功能优化**：修复搜索后翻页导致索引错位的问题
- **前端资源压缩**：所有 JS/CSS 文件自动压缩，减少带宽占用

#### 4. 🤖 CI/CD 自动化
- **自动发布流程**：配置 GitHub Actions 自动构建和发布新版本
- **跨平台构建**：支持 Windows、Linux、macOS 等多平台自动构建
- **Docker 集成**：将 esbuild 前端构建集成到所有 CI/CD 流程中
- **Dependabot 配置**：自动更新依赖项，保持项目安全性和稳定性

#### 5. 🔧 功能增强
- **自定义数据库路径**：添加 `-database` 命令行参数，支持自定义数据库文件路径
- **vnext 数组解析**：修复出站设置无法识别 vnext 数组中地址和端口的问题
- **帮助信息完善**：在命令行帮助中添加 `cert` 命令说明

#### 6. 📦 依赖项更新
- 更新至最新的依赖项版本：
  - `github.com/xtls/xray-core` 至 v1.260327.0
  - `github.com/gin-contrib/sessions` 至 v1.1.0
  - `google.golang.org/grpc` 至 v1.80.0
  - 以及其他多个依赖项

## 快速开始

```bash
bash <(curl -Ls https://raw.githubusercontent.com/masx200/3x-ui-fleet-hub/master/install.sh)
```

## 前端开发

本项目使用 esbuild 进行前端构建：

```bash
cd web

# 安装依赖
pnpm install

# 构建前端资源
pnpm build

# 清理构建产物
pnpm clean
```

## 开发命令

```bash
# 构建
go build -o bin/3x-ui.exe ./main.go

# 运行（带调试日志）
XUI_DEBUG=true go run ./main.go

# 测试
go test ./...

# Vet
go vet ./...
```

### 命令行参数

- `run` - 启动 Web 面板
- `migrate` - 从旧版 x-ui 迁移数据库
- `setting` - 修改面板设置（`-port`, `-username`, `-password`, `-webBasePath`, `-listenIP`, `-database`, `-reset`, `-show` 等）
- `cert` - 更新 SSL 证书

## 架构概览

### 目录结构

```
main.go                 # 入口文件，信号处理（SIGHUP/SIGTERM/SIGUSR1）
config/                 # 配置管理（嵌入版本/名称文件）
database/               # GORM 模型和 SQLite 初始化
  ├── model/model.go    # 所有数据库模型（含自动迁移）
web/                    # 主 Web 服务器（Gin）
  ├── controller/       # HTTP 处理器（使用 *gin.Context）
  ├── service/          # 业务逻辑层
  ├── job/              # 基于 Cron 的后台任务
  ├── entity/           # 请求/响应 DTO
  ├── middleware/       # Gin 中间件
  ├── locale/           # i18n 辅助函数
  ├── websocket/        # WebSocket Hub 用于实时更新
  ├── src/              # 前端源代码（Vue.js、CSS、JS）
  ├── build/            # 前端构建输出（esbuild 生成）
  └── scripts/          # 构建脚本（esbuild 配置）
xray/                   # Xray-core 进程管理和 gRPC API 客户端
sub/                    # 订阅服务器（运行在独立端口）
util/                   # 共享工具（加密、LDAP、系统信息）
```

### 核心架构模式

**嵌入资源**：所有 Web 资源在编译时使用 `//go:embed` 嵌入：
- `web/src` → esbuild 构建 → `web/build` → 嵌入到二进制文件

**双服务器设计**：两个服务器并发运行：
1. 主 Web 面板（可配置端口，默认 2053）
2. 订阅服务器（独立端口用于客户端订阅 URL）

**Xray 集成模式**：
- 面板从数据库的入站/出站动态生成 `config.json`
- Xray 二进制文件由安装脚本单独下载到 `{bin_folder}/xray-{os}-{arch}`
- 通过 gRPC API 进行实时流量统计通信
- 进程生命周期管理在 `xray/process.go` 中

**信号重启机制**：
- SIGHUP 触发 Web 和订阅服务器的优雅重启
- SIGUSR1 仅重启 Xray-core
- **关键**：重启前始终调用 `service.StopBot()` 以避免 Telegram bot 409 冲突

## 新增数据库模型

```go
// OutboundTestResult 存储每个出站的最新测试结果
type OutboundTestResult struct {
    Id         int    // 主键
    Tag        string // 出站标签（唯一索引）
    Success    bool   // 测试是否成功
    Delay      int64  // 延迟（毫秒）
    StatusCode int    // HTTP 状态码
    Error      string // 错误信息
    UpdatedAt  int64  // 更新时间
}
```

## 特别感谢

- [alireza0](https://github.com/alireza0/)
- [MHSanaei](https://github.com/MHSanaei/) - 原项目作者

## 致谢

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (License: **GPL-3.0**)：增强的 v2ray/xray 路由规则，内置伊朗域名，注重安全和广告拦截
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (License: **GPL-3.0**)：基于俄罗斯被封锁域名和地址数据自动更新的 V2Ray 路由规则

## 支持项目

如果本项目对您有帮助，请给我们一个⭐️

## 许可证

[GPL v3](https://www.gnu.org/licenses/gpl-3.0.en.html)

---

**Forked from** [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui)  
**Enhanced by** [masx200](https://github.com/masx200)
