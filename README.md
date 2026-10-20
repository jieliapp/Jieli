# Jieli

简体中文 · [English](README.en.md)

保存、搜索、分享和导出 AI 编程对话。支持 Claude Code、Codex，其他 Agent 待支持。

## Docker 部署

安装 Docker 和 Docker Compose，首次部署执行：

```sh
git clone https://github.com/jieliapp/Jieli.git
cd Jieli
docker compose up -d --build
```

打开 [http://localhost:8080](http://localhost:8080)，进入登录页，填写邮箱、密码和确认密码，即可创建管理员并自动登录，无需验证邮件。
请先完成管理员创建，再将站点开放给其他人。

管理员可直接创建用户，或在管理后台配置并测试 SMTP，再开启邮箱注册。
自托管版不限同步消息数量；站点地址和邮件设置均在管理后台配置。

数据库、附件和自动生成的 `.env` 都保存在 Docker 数据卷 `/app/data` 中，升级前请做好备份。不要执行 `docker compose down -v`，这会删除数据。

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
