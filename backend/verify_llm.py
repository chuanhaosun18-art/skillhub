# -*- coding: utf-8 -*-
"""验证 AI 个性化解读接口：注册带 ai_level 用户 -> explain skill -> 检查返回"""
import json
import time
import urllib.request
import urllib.error

BASE = "http://localhost:8080/api"


def req(method, path, body=None, token=None):
    url = BASE + path
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = "Bearer " + token
    r = urllib.request.Request(url, data=data, headers=headers, method=method)
    for _ in range(3):
        try:
            resp = urllib.request.urlopen(r)
            return resp.status, json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            try:
                return e.code, json.loads(e.read().decode("utf-8"))
            except Exception:
                return e.code, {}
        except ConnectionResetError:
            time.sleep(0.3)
    return -1, {}


def check(name, ok, detail=""):
    print(("PASS" if ok else "FAIL") + " | " + name + ((" | " + str(detail)) if detail else ""))


uname = "llmtest_" + str(int(time.time()))
code, data = req("POST", "/auth/register", {
    "username": uname,
    "email": uname + "@test.com",
    "password": "pass123456",
    "school": "北京交通大学",
    "major": "计算机科学与技术",
    "grade": "研二",
    "ai_level": "never",   # 从未用过 AI 工具
})
token = data.get("token", "") if code == 201 else ""
uid = data.get("user", {}).get("id") if code == 201 else None
check("注册带ai_level(201)", code == 201, (code, data))
check("注册返回ai_level", code == 201 and data.get("user", {}).get("ai_level") == "never", data.get("user"))

# 登录后 me 应带 ai_level
code, data = req("GET", "/auth/me", token=token)
check("me含ai_level", code == 200 and data.get("data", {}).get("ai_level") == "never", data)

# 游客调用 explain 应 401
code, data = req("GET", "/skills/5/explain")
check("游客explain(401)", code == 401, (code, data))

# 登录用户调用 explain（真实调用 DeepSeek，可能较慢）
print("\n-- 调用 DeepSeek 生成个性化介绍（可能需要 10-60 秒）--")
code, data = req("GET", "/skills/5/explain", token=token)
check("explain(200)", code == 200, (code, data))
if code == 200:
    content = data.get("data", "")
    check("返回内容非空", len(content) > 50, content[:100])
    check("返回level_label=从未用过", data.get("level_label") == "从未用过", data.get("level_label"))
    check("首次未命中缓存", data.get("cached") is False)
    print("\n--- 生成内容（截取前 400 字）---")
    print(content[:400])

# 第二次调用应命中缓存
code, data2 = req("GET", "/skills/5/explain", token=token)
check("二次命中缓存", code == 200 and data2.get("cached") is True, (code, data2))

# 更新 ai_level 为 advanced，生成内容应不同
code, data = req("PUT", f"/users/{uid}", {
    "ai_level": "advanced",
}, token=token)
check("更新ai_level", code == 200 and data.get("data", {}).get("ai_level") == "advanced", (code, data))

print("\n-- advanced 水平重新生成（新 key 缓存）--")
code, data = req("GET", "/skills/5/explain", token=token)
check("advanced explain(200)", code == 200, (code, data))
if code == 200:
    print("\n--- advanced 内容（截取前 400 字）---")
    print(data.get("data", "")[:400])

print("\n-- 清理 --")
# 删除测试用户（保留缓存不影响）
import sqlite3
db = sqlite3.connect(r"D:\skillhub-data\skillhub.db")
db.execute("DELETE FROM users WHERE username LIKE 'llmtest_%'")
db.commit()
db.close()
print("已清理测试用户")
