# -*- coding: utf-8 -*-
"""端到端验证 AI 引导创建 Skill：对话 -> 生成 skill 包 -> 解 base64 zip -> multipart 发布 -> 校验"""
import base64
import io
import json
import time
import zipfile
import urllib.request
import urllib.error

BASE = "http://localhost:8080/api"


def req(method, path, body=None, token=None, raw=None, ctype=None):
    url = BASE + path
    headers = {}
    data = None
    if raw is not None:
        data = raw
        if ctype:
            headers["Content-Type"] = ctype
    elif body is not None:
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
        except Exception as e:
            return -1, {"error": str(e)}
    return -1, {}


def check(name, ok, detail=""):
    print(("PASS" if ok else "FAIL") + " | " + name + ((" | " + str(detail)) if detail else ""))


# 1. 登录
code, data = req("POST", "/auth/login", {"account": "ev_guide_022149", "password": "test123456"})
if code != 200 or not data.get("token"):
    check("login", False, data)
    raise SystemExit(1)
token = data["token"]
check("login", True)

# 2. 引导对话（多轮，给足信息）
conv = [
    {"role": "user", "content": "我想做一个帮本科生整理保研经验的skill，包括材料准备、导师联系、复试技巧"},
    {"role": "assistant", "content": "好的！能具体说下材料准备阶段包含哪些内容吗？"},
    {"role": "user", "content": "材料准备包括成绩单、获奖证书整理、个人陈述、推荐信；导师联系包括怎么选导师、怎么写第一封邮件；复试技巧包括自我介绍、专业问题准备、英语面试。输出是分阶段的行动清单加模板。"},
]
t0 = time.time()
code, data = req("POST", "/skills/guide/chat", {"messages": conv}, token=token)
ok = code == 200 and data.get("data")
check("guide/chat 多轮对话", ok, "" if ok else data)
if ok:
    reply = data["data"].replace("\n", " ")[:60]
    print("       回复片段: " + reply + (" | 耗时%.1fs" % (time.time() - t0)))

# 3. 生成 skill 包
t0 = time.time()
conv.append({"role": "assistant", "content": reply})
code, data = req("POST", "/skills/guide/generate", {"messages": conv}, token=token)
gen = data.get("data", {}) if code == 200 else {}
ok = code == 200 and gen.get("name") and gen.get("zip_base64") and gen.get("files")
check("guide/generate 生成 skill 包", ok, "" if ok else data)
if ok:
    names = ", ".join(f["path"] for f in gen["files"])
    print("       name=%s title=%s | files: %s | zip_base64=%d chars | 耗时%.1fs"
          % (gen["name"], gen["title"], names, len(gen["zip_base64"]), time.time() - t0))
    # 4. 解 base64 为 zip，校验 zip 合法
    try:
        zdata = base64.b64decode(gen["zip_base64"])
        zf = zipfile.ZipFile(io.BytesIO(zdata))
        znames = zf.namelist()
        check("zip 解包合法", "SKILL.md" in znames, "zip 内文件: %s" % znames)
    except Exception as e:
        check("zip 解包合法", False, e)

    # 5. multipart 发布（模拟前端 handlePublishGenerated）
    boundary = "----skillhub-e2e-%d" % int(time.time())
    fields = {
        "name": gen["title"] or gen["name"],
        "description": gen.get("description", ""),
        "category": gen.get("category", "其他"),
        "tags": json.dumps(gen.get("tags", []), ensure_ascii=False),
        "version": gen.get("version", "1.0.0"),
    }
    buf = io.BytesIO()
    for k, v in fields.items():
        buf.write(("--%s\r\nContent-Disposition: form-data; name=\"%s\"\r\n\r\n%s\r\n" % (boundary, k, v)).encode("utf-8"))
    buf.write(("--%s\r\nContent-Disposition: form-data; name=\"archive\"; filename=\"%s.zip\"\r\nContent-Type: application/zip\r\n\r\n" % (boundary, gen["name"])).encode("utf-8"))
    buf.write(base64.b64decode(gen["zip_base64"]))
    buf.write(("\r\n--%s--\r\n" % boundary).encode("utf-8"))

    t0 = time.time()
    code, data = req("POST", "/skills", token=token, raw=buf.getvalue(),
                     ctype="multipart/form-data; boundary=%s" % boundary)
    ok = code in (200, 201) and data.get("data", {}).get("id")
    check("发布生成的 skill 包", ok, "" if ok else data)
    if ok:
        sid = data["data"]["id"]
        print("       skill id=%s | 耗时%.1fs" % (sid, time.time() - t0))
        # 6. 校验详情可查、文件已入库
        code, data = req("GET", "/skills/%s" % sid)
        d = data.get("data", {})
        check("详情可查", code == 200 and d.get("name"), d.get("name"))
