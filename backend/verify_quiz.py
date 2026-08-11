# -*- coding: utf-8 -*-
"""验证注册问卷 → ai_level 自动推导链路（不依赖真实 LLM，只测推导逻辑）"""
import json
import time
import urllib.request

BASE = "http://localhost:8080"
PASS = 0
FAIL = 0


def req(path, method="GET", body=None, token=None):
    """带 3 次重试的请求，返回 (status, parsed)"""
    data = json.dumps(body).encode() if body is not None else None
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    for _ in range(3):
        try:
            r = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
            with urllib.request.urlopen(r) as resp:
                return resp.status, json.loads(resp.read().decode())
        except urllib.error.HTTPError as e:
            try:
                return e.code, json.loads(e.read().decode())
            except Exception:
                return e.code, {}
        except Exception:
            time.sleep(1)
    return -1, {}


def check(name, cond, extra=""):
    global PASS, FAIL
    if cond:
        PASS += 1
        print(f"PASS | {name} | {extra}")
    else:
        FAIL += 1
        print(f"FAIL | {name} | {extra}")


def register(username, quiz):
    return req("/api/auth/register", "POST", {
        "username": username, "email": f"{username}@test.com", "password": "test123456",
        "school": "北京交通大学", "grade": "研二", "major": "计算机", "ai_quiz": quiz,
    })


def main():
    # 场景 1：全否 → never
    st, r = register(f"quiztest_{int(time.time())}_n", {
        "heard_of_llm": False, "used_llm": False, "used_agent": False,
        "has_agent_installed": False, "ran_full_project": False,
    })
    check("全否 → 注册成功", st == 201, f"{st}")
    check("全否 → ai_level=never", r.get("user", {}).get("ai_level") == "never", r.get("user", {}).get("ai_level"))
    check("全否 → ai_quiz 落库", "heard_of_llm" in (r.get("user", {}).get("ai_quiz") or ""), r.get("user", {}).get("ai_quiz", "")[:60])

    # 场景 2：听过 + 用过 LLM，没用 agent → beginner
    st, r = register(f"quiztest_{int(time.time())}_b", {
        "heard_of_llm": True, "used_llm": True, "used_agent": False,
        "has_agent_installed": False, "ran_full_project": False,
    })
    check("用过LLM无Agent → beginner", st == 201 and r.get("user", {}).get("ai_level") == "beginner", r.get("user", {}).get("ai_level"))

    # 场景 3：+ 用过 agent，没跑完整项目 → intermediate
    st, r = register(f"quiztest_{int(time.time())}_i", {
        "heard_of_llm": True, "used_llm": True, "used_agent": True,
        "has_agent_installed": True, "ran_full_project": False,
    })
    check("用过Agent未跑项目 → intermediate", st == 201 and r.get("user", {}).get("ai_level") == "intermediate", r.get("user", {}).get("ai_level"))

    # 场景 4：+ 跑过完整项目 → advanced
    st, r = register(f"quiztest_{int(time.time())}_a", {
        "heard_of_llm": True, "used_llm": True, "used_agent": True,
        "has_agent_installed": True, "ran_full_project": True,
    })
    check("用过Agent跑过项目 → advanced", st == 201 and r.get("user", {}).get("ai_level") == "advanced", r.get("user", {}).get("ai_level"))
    uid = r.get("user", {}).get("id")
    token = r.get("token")

    # me 返回 ai_quiz
    st, r = req("/api/auth/me", token=token)
    check("me 返回 ai_quiz", st == 200 and "ran_full_project" in (r.get("data", {}).get("ai_quiz") or ""), r.get("data", {}).get("ai_quiz", "")[:80])

    # updateUser 重答问卷 → 重新推导（advanced 用户改答为 beginner 场景）
    st, r = req(f"/api/users/{uid}", "PUT", {
        "ai_quiz": {"heard_of_llm": True, "used_llm": True, "used_agent": False,
                    "has_agent_installed": False, "ran_full_project": False},
    }, token=token)
    check("重答问卷 → 重新推导", st == 200 and r.get("data", {}).get("ai_level") == "beginner", r.get("data", {}).get("ai_level"))

    # updateUser 不带水平字段 → 保留原值不覆盖
    st, r = req(f"/api/users/{uid}", "PUT", {"bio": "改简介而已"}, token=token)
    check("普通更新不覆盖 ai_level", st == 200 and r.get("data", {}).get("ai_level") == "beginner", r.get("data", {}).get("ai_level"))
    check("普通更新保留 ai_quiz", "used_agent" in (r.get("data", {}).get("ai_quiz") or ""), r.get("data", {}).get("ai_quiz", "")[:60])

    # 清理测试用户（删除测试账号）
    for suffix in ["_n", "_b", "_i", "_a"]:
        # 用用户名模糊查不到删除接口，这里通过注册列表——简化：直接忽略，测试账号由 sqlite 手动清理
        pass

    print(f"\n== 结果：{PASS} PASS, {FAIL} FAIL ==")
    raise SystemExit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
