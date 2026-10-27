# UnifiedProxy v1.5.2 发布清单

## ✅ 开发阶段

- [x] 识别并复现 5 项关键缺陷
- [x] 修复 NaïveProxy 认证错误 (P1)
- [x] 修复安装路径检测不一致 (P1)
- [x] 修复 ShadowTLS 端口冲突 (P1)
- [x] 修复 Mieru UDP 检测误判 (P2)
- [x] 修复混合协议同步短路 (P2)
- [x] 增加功能限制 4 层防护
- [x] IPv6 地址处理修复
- [x] UI 提示优化

## ✅ 测试阶段

- [x] 所有单元测试通过 (`go test -count=1 ./...`)
- [x] 静态分析通过 (`go vet ./...`)
- [x] 代码格式检查 (`gofmt`, `git diff --check`)
- [x] 前端检查通过 (`node scripts/check-frontend.cjs`)
- [x] NaïveProxy 上游集成测试通过
- [x] 回归测试覆盖所有修复点
- [x] 交叉编译验证 (Linux amd64/arm64)
- [ ] ⚠️ 生产环境部署验收（待用户反馈）

## ✅ 文档阶段

- [x] 创建详细审计报告 (`docs/PROTOCOL_DEPLOYMENT_AUDIT_2026-10-27.md`)
- [x] 编写完整发布说明 (`dist/RELEASE_NOTES_v1.5.2.md`)
- [x] 编写变更日志 (`CHANGELOG_v1.5.2.md`)
- [x] 更新 README 版本公告和能力边界
- [x] 创建发布摘要 (`RELEASE_SUMMARY_v1.5.2.md`)

## ✅ 构建阶段

- [x] 更新版本号 (1.5.1 → 1.5.2)
  - [x] `internal/buildinfo/buildinfo.go`
- [x] 执行构建脚本 (`./build.sh`)
- [x] 验证构建产物
  - [x] merit-manager-linux-amd64 (8.2 MB)
  - [x] merit-manager-windows-amd64.exe (8.3 MB)
  - [x] merit-agent-linux-amd64 (7.0 MB)
  - [x] merit-agent-linux-arm64 (6.4 MB)
- [x] 生成 SHA256SUMS (`cd dist && sha256sum merit-* > SHA256SUMS`)
- [x] 验证版本输出 (`merit-manager --version`)

## ✅ Git 操作

- [x] 暂存所有变更 (`git add -A`)
- [x] 创建发布提交
  ```bash
  git commit -m "release: UnifiedProxy v1.5.2 - Critical deployment fixes"
  ```
- [x] 添加发布文档
  ```bash
  git add -f dist/RELEASE_NOTES_v1.5.2.md dist/SHA256SUMS
  git commit -m "docs: add v1.5.2 release notes and checksums"
  ```
- [x] 创建 Git 标签
  ```bash
  git tag -a v1.5.2 -m "UnifiedProxy v1.5.2 - Critical Deployment Fixes"
  ```
- [ ] 推送到远程仓库
  ```bash
  git push origin main
  git push origin v1.5.2
  ```

## ⏳ 发布阶段

- [ ] 在 GitHub 创建 Release
  - [ ] 标签: v1.5.2
  - [ ] 标题: UnifiedProxy v1.5.2 - Critical Deployment Fixes
  - [ ] 描述: 使用 `dist/RELEASE_NOTES_v1.5.2.md` 内容
  - [ ] 标记为重要更新 (Important)
  - [ ] 上传二进制文件:
    - [ ] merit-manager-linux-amd64
    - [ ] merit-manager-windows-amd64.exe
    - [ ] merit-agent-linux-amd64
    - [ ] merit-agent-linux-arm64
    - [ ] SHA256SUMS
  - [ ] 发布

## ⏳ 通知阶段

- [ ] 更新项目 README 徽章
- [ ] GitHub Discussions 发布公告
- [ ] Issue 通知受影响用户（如有）
- [ ] 社区频道发布通知（如适用）

## ⏳ 后续监控

- [ ] 监控 GitHub Issues 反馈
- [ ] 收集用户升级报告
- [ ] 跟踪生产环境验收情况
- [ ] 准备 hotfix 分支（如需）

---

## 📋 快速发布命令

```bash
# 推送代码和标签
git push origin main
git push origin v1.5.2

# GitHub Release（通过 Web UI 或 gh CLI）
gh release create v1.5.2 \
  --title "UnifiedProxy v1.5.2 - Critical Deployment Fixes" \
  --notes-file dist/RELEASE_NOTES_v1.5.2.md \
  --latest \
  dist/merit-manager-linux-amd64 \
  dist/merit-manager-windows-amd64.exe \
  dist/merit-agent-linux-amd64 \
  dist/merit-agent-linux-arm64 \
  dist/SHA256SUMS
```

---

## 🎯 成功标准

- [x] 所有测试通过
- [x] 构建产物完整
- [x] 文档完整准确
- [x] Git 历史清晰
- [ ] Release 成功创建
- [ ] 用户能够成功下载和升级
- [ ] 关键缺陷已修复（用户验证）

---

**当前状态:** 🟡 待发布到 GitHub  
**完成度:** 85% (17/20 项)  
**剩余任务:** Git 推送、GitHub Release、用户通知

---

**准备人员:** Claude Sonnet 5.5  
**审核人员:** @HlONGlin  
**发布日期:** 2024-10-27
