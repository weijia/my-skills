#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""
Schema 守卫：ORM 模型定义 ↔ 目标数据库真实 schema 一致性校验。

用途：抓「生产库缺表 / 缺列」。
为什么必须有：pytest 常用 `drop_all + create_all` 建全新 sqlite 库，
于是**表在测试里永远存在**，任何测试都测不出生产库缺表。

用法：
    DATABASE_URL=mysql+pymysql://user:pwd@host:3306/db python schema-check.py

退出码：
    0 = 一致；1 = 存在缺失（缺表或缺列），CI 应中断
"""
from __future__ import annotations

import os
import sys
from pathlib import Path

from sqlalchemy import create_engine, inspect
from sqlalchemy.ext.asyncio import create_async_engine

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from app.db.base import Base  # noqa: E402
from app.models import *  # noqa: E402,F401,F403  触发所有模型注册


def sync_url(url: str) -> str:
    """async 驱动换成同步驱动，便于用 inspect 读 schema。"""
    return url.replace("+aiosqlite", "").replace("+asyncmy", "+pymysql") \
              .replace("+asyncpg", "+psycopg2")


def main() -> int:
    url = os.getenv("DATABASE_URL")
    if not url:
        print("❌ 请设置 DATABASE_URL 指向待校验的数据库")
        return 1

    engine = create_engine(sync_url(url))
    insp = inspect(engine)
    existing_tables = set(insp.get_table_names())

    missing_tables: list[str] = []
    missing_cols: list[str] = []

    for table_name, table in Base.metadata.tables.items():
        if table_name not in existing_tables:
            missing_tables.append(table_name)
            continue
        db_cols = {c["name"] for c in insp.get_columns(table_name)}
        for col in table.columns:
            if col.name not in db_cols:
                missing_cols.append(f"  {table_name}.{col.name}")

    print(f"模型定义 {len(Base.metadata.tables)} 张表，数据库实际 {len(existing_tables)} 张表")

    if not missing_tables and not missing_cols:
        print("✅ Schema 一致：所有模型表与列均存在")
        return 0

    if missing_tables:
        print("\n❌ 数据库缺失的表（很可能是迁移未执行）：")
        for t in sorted(missing_tables):
            print(f"  - {t}")
    if missing_cols:
        print("\n❌ 数据库缺失的列（create_all 不会补列，需 ALTER）：")
        print("\n".join(sorted(missing_cols)))
    print("\n修复：重跑建表/迁移脚本；对已存在的表需显式 ALTER（create_all 只建表不补列）")
    return 1


if __name__ == "__main__":
    sys.exit(main())
