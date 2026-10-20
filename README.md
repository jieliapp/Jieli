# Jieli

简体中文 · [English](README.en.md)

[![Go CI](https://img.shields.io/github/actions/workflow/status/jieliapp/Jieli/go.yml?branch=main&style=flat-square&logo=github&label=Go%20CI)](https://github.com/jieliapp/Jieli/actions/workflows/go.yml)
[![License](https://img.shields.io/github/license/jieliapp/Jieli?style=flat-square)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![Svelte](https://img.shields.io/badge/Svelte-5-FF3E00?style=flat-square&logo=svelte&logoColor=white)](https://svelte.dev/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker&logoColor=white)](https://docs.docker.com/compose/)

Jieli 为 AI 编程对话提供**云端同步、备份、搜索和分享**：将 Claude Code、Codex 的本地会话实时同步到自己的服务器，方便跨设备查看、全文搜索和链接分享。

支持同步历史会话和导出对话，数据由你掌控；其他 Agent 待支持。

## Docker 部署

安装 Docker 和 Docker Compose，首次部署执行：

```sh
git clone https://github.com/jieliapp/Jieli.git
cd Jieli
docker compose up -d --build
```

启动后访问 [http://localhost:8080](http://localhost:8080)，按提示创建管理员即可使用，数据持久化保存在 `/app/data` 数据卷中，升级前请备份。

## 非 Docker 部署

适用于 Linux/macOS，脚本会检查所需依赖并提示缺失项。

```sh
git clone https://github.com/jieliapp/Jieli.git
cd Jieli
./scripts/run.sh
```

脚本自动构建并启动服务，访问 [http://localhost:8080](http://localhost:8080)。数据库、附件和自动生成的 `.env` 保存在 `data/`，升级前请备份此目录。服务在前台运行，按 `Ctrl+C` 停止；更新代码后再次运行同一脚本即可。

## 对话同步

安装 Agent 对应的 [Jieli 插件](https://github.com/jieliapp/plugins)，将对话实时同步到自己的实例。

同步历史的 Claude Code、Codex 会话（需 Node.js 20+）：

```sh
curl -fsSLo jieli-sync.mjs https://jieli.app/jieli-sync.mjs && node jieli-sync.mjs
```

脚本沿用插件配置，缺少配置时会提示填写。自托管用户请将 `JIELI_BASE_URL` 或 `~/.config/jieli/settings.json` 中的 `base_url` 设置为自己的站点地址。

## 许可证

本项目采用 [Apache License 2.0](LICENSE)。
