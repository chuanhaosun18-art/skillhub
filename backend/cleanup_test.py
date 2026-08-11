# -*- coding: utf-8 -*-
"""清理 auth 测试产生的残留数据（tester_ 用户及其技能）"""
import sqlite3

db = sqlite3.connect(r"D:\skillhub-data\skillhub.db")
cur = db.cursor()

# 删除 tester_ 用户发布的技能（含无 archive 的测试 skill id=6）
cur.execute("DELETE FROM skills WHERE owner_id IN (SELECT id FROM users WHERE username LIKE 'tester_%')")
deleted_skills = cur.rowcount

# 删除 tester_ 用户
cur.execute("DELETE FROM users WHERE username LIKE 'tester_%'")
deleted_users = cur.rowcount

# 删除这些技能残留的文件目录/归档（无 archive 的测试数据本来就没有）
db.commit()

cur.execute("SELECT id, name, owner_id FROM skills ORDER BY id")
rows = cur.fetchall()
cur.execute("SELECT id, username FROM users ORDER BY id")
users = cur.fetchall()
db.close()

print(f"已删除测试技能: {deleted_skills} 条")
print(f"已删除测试用户: {deleted_users} 条")
print("剩余技能:", rows)
print("剩余用户:", users)
