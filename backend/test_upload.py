# 用标准库发送 multipart/form-data，模拟前端上传 skill 的行为
import json
import mimetypes
import os
import urllib.request
import uuid

BASE = "http://localhost:8080"


def multipart_post(url, fields, file_field, file_path):
    boundary = uuid.uuid4().hex
    lines = []
    for k, v in fields.items():
        lines.append(f"--{boundary}".encode())
        lines.append(f'Content-Disposition: form-data; name="{k}"'.encode())
        lines.append(b"")
        lines.append(str(v).encode("utf-8"))
    if file_field and file_path:
        lines.append(f"--{boundary}".encode())
        lines.append(
            f'Content-Disposition: form-data; name="{file_field}"; filename="{os.path.basename(file_path)}"'.encode()
        )
        lines.append(f"Content-Type: {mimetypes.guess_type(file_path)[0] or 'application/octet-stream'}".encode())
        lines.append(b"")
        with open(file_path, "rb") as f:
            lines.append(f.read())
    lines.append(f"--{boundary}--".encode())
    lines.append(b"")
    body = b"\r\n".join(lines)

    req = urllib.request.Request(
        BASE + url,
        data=body,
        headers={"Content-Type": f"multipart/form-data; boundary={boundary}"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        body_text = e.read().decode("utf-8", errors="replace")
        print(f"[!] HTTP {e.code} body: {body_text}")
        raise


def get(url):
    with urllib.request.urlopen(BASE + url) as resp:
        return resp.status, json.loads(resp.read().decode("utf-8"))


if __name__ == "__main__":
    fields = {
        "name": "北京交通大学论文写作Skill",
        "description": "基于BJTU LaTeX模板的论文自动写作技能包，包含SKILL.md与脚本。",
        "category": "学术写作",
        "tags": json.dumps(["LaTeX", "论文", "BJTU"], ensure_ascii=False),
        "version": "1.0.0",
    }
    status, body = multipart_post("/api/skills", fields, "archive", r"D:\skillhub-test\test-skill.zip")
    print("create status:", status)
    print(json.dumps(body, ensure_ascii=False, indent=2))

    if status == 201:
        sid = body["data"]["id"]
        status, detail = get(f"/api/skills/{sid}")
        print("\n--- detail ---")
        print(json.dumps(detail, ensure_ascii=False, indent=2))

        status, search = get("/api/skills?keyword=%E8%AE%BA%E6%96%87")  # 论文
        print("\n--- search '论文' ---")
        print(json.dumps(search, ensure_ascii=False, indent=2))
