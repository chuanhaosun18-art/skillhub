# -*- coding: utf-8 -*-
"""验证认证链路：注册 -> 登录 -> me -> 发布skill -> 我的技能 -> 游客权限"""
import json
import urllib.request
import urllib.error
import time

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
            time.sleep(0.3)  # 偶发连接重置，重试
    return -1, {"error": "connection reset"}


def check(name, ok, detail=""):
    print(("PASS" if ok else "FAIL") + " | " + name + ((" | " + str(detail)) if detail else ""))


# 1. 注册新用户（带学校/年级/专业/简介）
uname = "tester_" + str(int(time.time()))
reg_body = {
    "username": uname,
    "email": uname + "@test.com",
    "password": "pass123456",
    "school": "北京交通大学",
    "grade": "研二",
    "major": "计算机科学与技术",
    "bio": "测试用户",
}
code, data = req("POST", "/auth/register", reg_body)
token = data.get("token", "") if code == 201 else ""
check("注册成功(201)", code == 201, (code, data))
if code == 201:
    u = data["user"]
    check("画像字段: school", u.get("school") == "北京交通大学", u)
    check("画像字段: grade", u.get("grade") == "研二", u)
    check("画像字段: major", u.get("major") == "计算机科学与技术", u)
    check("画像字段: bio", u.get("bio") == "测试用户", u)

# 2. 重复注册应 409
code2, data2 = req("POST", "/auth/register", reg_body)
check("重复注册(409)", code2 == 409, (code2, data2))

# 3. 登录（用户名）
code, data = req("POST", "/auth/login", {"account": uname, "password": "pass123456"})
check("登录成功-用户名(200)", code == 200 and "token" in data, (code, data))
token = data.get("token", "")

# 4. 登录（邮箱）
code, data = req("POST", "/auth/login", {"account": uname + "@test.com", "password": "pass123456"})
check("登录成功-邮箱(200)", code == 200, (code, data))

# 5. 错误密码 401
code, data = req("POST", "/auth/login", {"account": uname, "password": "wrongpass"})
check("错误密码(401)", code == 401, (code, data))

# 6. me
code, data = req("GET", "/auth/me", token=token)
check("me(200)", code == 200 and data.get("data", {}).get("username") == uname, (code, data))

# 7. 无 token 访问 me 应 401
code, data = req("GET", "/auth/me")
check("无token访问me(401)", code == 401, (code, data))

# 8. 游客发布 skill 应 401
code, data = req("POST", "/skills", {"name": "should-not-exist", "description": "x", "category": "其他"})
check("游客发布skill(401)", code == 401, (code, data))

# 9. 登录用户发布 skill（multipart，与 createSkill 一致）
import io
import uuid

boundary = "----VerifyAuthBoundary" + uuid.uuid4().hex
fields = [
    ("name", "认证测试Skill_" + uname),
    ("description", "验证发布链路"),
    ("category", "测试"),
    ("tags", '["auth","test"]'),
    ("version", "1.0.0"),
]
buf = io.BytesIO()
for k, v in fields:
    buf.write(("--%s\r\nContent-Disposition: form-data; name=\"%s\"\r\n\r\n%s\r\n" % (boundary, k, v)).encode("utf-8"))
buf.write(("--%s--\r\n" % boundary).encode("utf-8"))
r = urllib.request.Request(BASE + "/skills", data=buf.getvalue(), method="POST",
                           headers={"Content-Type": "multipart/form-data; boundary=" + boundary,
                                    "Authorization": "Bearer " + token})
try:
    resp = urllib.request.urlopen(r)
    data = json.loads(resp.read().decode("utf-8"))
    code = resp.status
except urllib.error.HTTPError as e:
    code = e.code
    data = json.loads(e.read().decode("utf-8"))
skill_id = data.get("data", {}).get("id") if code == 201 else None
check("登录发布skill(201)", code == 201, (code, data))

# 10. 我的技能列表
code, data = req("GET", "/users/me/skills", token=token)
found = any(s.get("id") == skill_id for s in data.get("data", []))
check("我的技能列表包含新skill", code == 200 and found, (code, data))

# 11. 详情页 owner 展示
if skill_id:
    code, data = req("GET", "/skills/" + str(skill_id))
    d = data.get("data", {})
    check("详情owner_name", code == 200 and d.get("owner_name") == uname, (code, d))

# 12. 删除测试 skill（避免污染数据）
if skill_id:
    code, data = req("DELETE", "/skills/" + str(skill_id), token=token)
    check("删除测试skill(200)", code == 200, (code, data))

print("\n-- 清理：删除测试用户与残留数据 --")
# 直接通过 sqlite 命令行删除，或留着也无妨；这里仅提示
