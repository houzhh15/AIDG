# AIDG v2 兼容性基线

本目录是阶段 0 的自动化兼容测试输入，默认不依赖 Docker、网络、ASR 或真实数据目录。

## 数据分级

当前仓库内会议样本全部是 **synthetic（合成脱敏样本）**，不是生产数据，也不能作为“已通过真实旧部署验收”的证据。它们只用于固定旧目录布局、文件保留规则和 API contract。

规划要求的真实旧部署样本必须由运维在仓库外完成脱敏、密钥扫描和授权审查后导入，并在 `manifest.json` 中标记为 `redacted-production`。至少记录：来源版本、脱敏人和时间、脱敏方法、是否包含音频、是否完成 secret/PII 审查、原始文件是否只保留在受控存储、每个文件 SHA-256。

严禁将未脱敏的会议文本、用户名、邮箱、JWT、API Key、LDAP/OIDC Secret、主机名、内网地址或绝对路径提交到 Git；也禁止先提交再从后续 commit 删除。

## 样本

- `meetings/synthetic-lite-minimal`：最小旧会议，只有 transcript 和摘要文件。
- `meetings/synthetic-standard-chunks`：旧分块、合并文本和润色文件。
- `meetings/synthetic-full-speaker-artifacts`：旧 speaker/NLP artifact，验证升级过程不删除未知历史文件。
- `server_tasks/`：已确认 wrapper schema 与两个路径兼容变体。顶层数组若未从真实历史版本确认，不纳入“已支持”结论。
- `compose/`：当前仓库 Lite/Standard/Full 部署文件的 reference snapshot；不冒充真实客户部署文件。
- `contracts/legacy-api-v1.json`：需要保持的旧 API 最小 contract 清单。

## 回滚规则

兼容测试只在临时副本执行。用户密码首次成功登录后会从旧 SHA-256 升级为 bcrypt；旧版本不能识别新 hash，因此二进制回滚时必须同时恢复升级前的 `users.json.bak`。


## 阶段 2 迁移写屏障

Meeting v2 使用 `<meetings-root>/.meetingv2-service.lock` 对共享数据根实施单实例独占写入。第二个新版服务实例会启动失败，不允许两个进程共享同一文件存储。

旧版本不识别该锁。因此旧版升级到包含阶段 2 迁移的新版本时，必须执行停机替换：停止并确认旧容器退出和数据卷无写入者，再启动新容器；禁止 rolling update、先启动新实例再停止旧实例，或让多个副本挂载同一 meetings root。迁移失败时先停止新容器，再恢复完整数据备份和旧镜像。仅回滚镜像而不恢复数据不属于受支持的回滚流程。