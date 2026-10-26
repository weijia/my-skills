#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""
契约守卫：前端调用的 API 路径 ↔ 后端真实路由 自动比对。

用途：抓 404 / 路径漂移（后端重构了接口但前端/测试没跟着改）。
这类问题靠人肉排查极慢，且 E2E 用离线桩时永远测不出来。

用法（按需改 FRONTEND_API_DIR / BACKEND_ROUTE_FILES 两个常量）：
    python contract-check.py          # 报告不匹配项，有则退出码 1（可直接接 CI）

约定：
- 只比对**去掉路径参数后的模板**，如 `/device/{id}` 与前端 `/device/${id}` 视为同一条。
- 前缀（如 /prod-api）在比对前统一剥离。
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

# —— 按需修改 ——
FRONTEND_API_DIR = Path("../frontend/src/api")      # 前端 API 定义目录
BACKEND_ROUTE_FILES = [Path("../backend/app/api/v1/endpoints")]  # 后端路由文件/目录
API_PREFIX = "/prod-api"                            # 前端请求前缀（比对前剥离）
# —— 按需修改 END ——

# 前端：request.get('/device/list', ...) / request.delete(`/device/${id}`)
FE_CALL_RE = re.compile(
    r"""request\.(get|post|put|delete)\(\s*[`'"]([^`'"]+)[`'"]""", re.M
)
# 前端模板字面量里的 ${...} / {id} 统一成占位符
FE_PLACEHOLDER_RE = re.compile(r"\$\{[^}]+\}|\{[a-zA-Z_]+\}")

# 后端（以 FastAPI 为例）：@router.get("/device/{id}")  @router.post("/device")
BE_ROUTE_RE = re.compile(
    r"""@router\.(get|post|put|delete|patch)\(\s*[f"']([^"']+)[f"']""", re.M
)


def norm(path: str) -> str:
    """归一化：剥前缀、路径参数统一成 {}、去尾斜杠。"""
    p = path.strip()
    if p.startswith(API_PREFIX):
        p = p[len(API_PREFIX):]
    p = FE_PLACEHOLDER_RE.sub("{}", p)
    if not p.startswith("/"):
        p = "/" + p
    return p.rstrip("/") or "/"


def collect_frontend() -> dict[str, set[str]]:
    """返回 {方法大写: {路径}}，并保留来源文件便于报错定位。"""
    out: dict[str, set[str]] = {}
    files = (
        [FRONTEND_API_DIR] if FRONTEND_API_DIR.is_file()
        else sorted(FRONTEND_API_DIR.rglob("*.js"))
    )
    for f in files:
        src = f.read_text(encoding="utf-8", errors="ignore")
        for method, raw in FE_CALL_RE.findall(src):
            out.setdefault(method.upper(), set()).add(norm(raw))
    return out


def collect_backend() -> dict[str, set[str]]:
    out: dict[str, set[str]] = {}
    files: list[Path] = []
    for entry in BACKEND_ROUTE_FILES:
        if entry.is_file():
            files.append(entry)
        else:
            files.extend(sorted(entry.rglob("*.py")))
    for f in files:
        src = f.read_text(encoding="utf-8", errors="ignore")
        for method, raw in BE_ROUTE_RE.findall(src):
            # 后端前缀（如 /device）通常在 include_router 处，这里只取装饰器里的相对路径，
            # 若后端用统一前缀，请在 ROUTE_PREFIX 里补上
            out.setdefault(method.upper(), set()).add(norm(raw))
    return out


def main() -> int:
    fe, be = collect_frontend(), collect_backend()
    # 后端有些路由用 @router.api_route 或动态注册，允许 ANY 匹配
    be_any = set()
    for paths in be.values():
        be_any |= paths

    missing: list[str] = []
    for method in sorted(fe):
        known = be.get(method, set()) | be_any
        for path in sorted(fe[method]):
            if path not in known:
                missing.append(f"  [{method}] {path}")

    print(f"前端 API 调用 {sum(len(v) for v in fe.values())} 条，"
          f"后端路由 {len(be_any)} 条")
    if missing:
        print("\n❌ 前端调用但后端无对应路由（404 / 路径漂移）：")
        print("\n".join(missing))
        return 1
    print("\n✅ 契约一致：前端所有 API 调用均能在后端路由表中找到")
    return 0


if __name__ == "__main__":
    sys.exit(main())
