# WowSkillLand 前端（迷茫期路由器）

静态页 Demo，对接本仓库 Go 后端。

## 启动

```bash
# 后端
cd backend
export SKILLHUB_DATA="$PWD/../.data"
export DEEPSEEK_API_KEY="sk-..."
go run .          # 默认 http://localhost:8080

# 前端
cd ../wowskillland-app
python3 -m http.server 5500
# 打开 http://localhost:5500
```

登录后，首页输入会走 `POST /api/growth/goals/interpret`（DeepSeek 三态路由）。
