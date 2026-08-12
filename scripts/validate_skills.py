#!/usr/bin/env python3
"""校验仓库 skills/ 目录下所有 skill.json 的格式。

在 GitHub Actions 的 PR 检查中运行：
- 每个技能子目录必须包含 skill.json，且必须是合法 JSON
- name 必须非空（导入工具以 name 判重）
- task_intent 若填写，必须在允许列表内（与后端 AllowedIntents 一致）
- 以下划线开头的目录（_template）与 README.md 不校验、也不视为技能

用法: python scripts/validate_skills.py [skills_dir]   # 默认仓库根的 skills/
"""
import json
import os
import sys

# 与后端 growth_db.go 的 AllowedIntents 保持一致
ALLOWED_INTENTS = {
    "thesis_topic", "resume_rewrite", "resume_jd_align", "report_structure",
    "mock_interview", "interview_review", "project_convergence",
    "literature_review", "content_script",
}

SKILLS_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "skills")
if len(sys.argv) > 1:
    SKILLS_DIR = sys.argv[1]


def main() -> int:
    if not os.path.isdir(SKILLS_DIR):
        print(f"skills 目录不存在: {SKILLS_DIR}")
        return 1

    errors = []
    warnings = []
    seen_names = set()
    skill_count = 0

    for entry in sorted(os.listdir(SKILLS_DIR)):
        full = os.path.join(SKILLS_DIR, entry)
        if not os.path.isdir(full) or entry.startswith((".", "_")):
            continue  # 非技能目录 / 模板 / 隐藏目录，跳过

        meta_path = os.path.join(full, "skill.json")
        if not os.path.isfile(meta_path):
            errors.append(f"[{entry}] 缺少 skill.json")
            continue
        try:
            with open(meta_path, encoding="utf-8") as f:
                meta = json.load(f)
        except (OSError, json.JSONDecodeError) as e:
            errors.append(f"[{entry}] skill.json 不是合法 JSON: {e}")
            continue
        if not isinstance(meta, dict):
            errors.append(f"[{entry}] skill.json 必须是 JSON 对象")
            continue

        name = str(meta.get("name", "")).strip()
        if not name:
            errors.append(f"[{entry}] skill.json 缺少 name（导入工具以 name 判重，必填）")
        elif name in seen_names:
            errors.append(f"[{entry}] name 与其它技能重复: {name}")
        else:
            seen_names.add(name)

        intent = meta.get("task_intent", "")
        if intent and intent not in ALLOWED_INTENTS:
            warnings.append(f"[{entry}] task_intent 不在允许列表，导入时会忽略: {intent}")

        skill_count += 1

    print(f"检查了 {skill_count} 个技能目录")
    for w in warnings:
        print(f"  [警告] {w}")
    for e in errors:
        print(f"  [错误] {e}")

    if errors:
        print("校验失败：请修复上述错误后再合入 PR。")
        return 1
    if warnings:
        print("校验通过（有警告，可合入）。")
    else:
        print("校验通过。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
