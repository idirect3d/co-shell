#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""AgentBenchmark Skill — 任务导出脚本（脱敏核心）。

将被测 agent 可执行的任务从 benchmark 库导出为"公开任务包"：
    - 剥离 reference（参考答案）与 scoring（评分细则）——防止被测 agent 作弊
    - 剥离测试文件与参考实现（code_test 任务）——保证公平
    - 仅保留任务指令、难度、类别、业务输入素材

用法:
    python scripts/export_tasks.py --benchmark ../../ --out run_20260830_120000
    python scripts/export_tasks.py --benchmark ../../ --tasks reins-001,gen-001 --out run_x

输出结构（<out>/tasks_public/）:
    INSTRUCTIONS.md          被测 agent 总清单（任务表 + 输出规范 + 红线）
    <task_id>/task.json      单任务公开版
    <task_id>/assets/        业务输入素材（已过滤 reference/test 文件）
"""
import json
import os
import shutil
import sys

SKILL_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def find_benchmark_root(start):
    """从 start 向上探测 benchmark 库根（含 schema/task.schema.json 与 tasks/）。"""
    cur = os.path.abspath(start)
    while True:
        if os.path.isdir(os.path.join(cur, "tasks")) and os.path.isfile(os.path.join(cur, "schema", "task.schema.json")):
            return cur
        parent = os.path.dirname(cur)
        if parent == cur:
            return None
        cur = parent


def collect_tasks(root):
    """扫描全部 .task.json，返回 {task_id: 路径}。"""
    found = {}
    for dirpath, _, files in os.walk(os.path.join(root, "tasks")):
        for f in sorted(files):
            if f.endswith(".task.json"):
                p = os.path.join(dirpath, f)
                with open(p, "r", encoding="utf-8") as fh:
                    t = json.load(fh)
                found[t["task_id"]] = p
    return found


def hidden_files(task):
    """返回必须对被测 agent 隐藏的文件（绝对路径集合）：reference.file + 各 criteria.test_file。"""
    ref = task.get("reference", {}) or {}
    hidden = set()
    if ref.get("file"):
        hidden.add(os.path.normpath(ref["file"]))
    for c in task.get("scoring", {}).get("criteria", []):
        if c.get("type") == "code_test" and c.get("test_file"):
            hidden.add(os.path.normpath(c["test_file"]))
    return hidden


def export_task(task_path, root, out_public):
    """导出单个任务的公开版，返回 (task_id, public_task, note)。"""
    with open(task_path, "r", encoding="utf-8") as fh:
        task = json.load(fh)
    tid = task["task_id"]
    hidden = hidden_files(task)
    hidden_basenames = {os.path.basename(h) for h in hidden}

    public = {
        "task_id": tid,
        "version": task.get("version", ""),
        "category": task.get("category", ""),
        "domain": task.get("domain", ""),
        "title": task.get("title", ""),
        "difficulty": task.get("difficulty", 1),
        "description": task.get("description", ""),
        "expected_output": task["expected_output"].get("description", ""),
        "input": {"type": task["input"].get("type", "text"), "text": task["input"].get("text", "")},
    }

    # 导出素材：input.files 中未被隐藏的文件 → assets/
    assets_dir = os.path.join(out_public, tid, "assets")
    os.makedirs(assets_dir, exist_ok=True)
    copied = []
    for f in task.get("input", {}).get("files", []):
        if os.path.normpath(f) in hidden or os.path.basename(f) in hidden_basenames:
            continue
        src = os.path.join(root, f)
        if not os.path.exists(src):
            continue
        dst = os.path.join(assets_dir, os.path.basename(f))
        if os.path.isdir(src):
            shutil.copytree(src, dst, dirs_exist_ok=True)
        else:
            shutil.copy2(src, dst)
        copied.append(os.path.basename(f))

    # 也导出 workdir 中未被隐藏的业务素材（过滤 reference/test）
    workdir = task.get("input", {}).get("workdir", "")
    if workdir:
        wsrc = os.path.join(root, workdir)
        if os.path.isdir(wsrc):
            for item in sorted(os.listdir(wsrc)):
                if item.endswith(".task.json"):
                    continue
                s = os.path.join(wsrc, item)
                if os.path.basename(s) in hidden_basenames or os.path.normpath(os.path.join(workdir, item)) in hidden:
                    continue
                dst = os.path.join(assets_dir, item)
                if not os.path.exists(dst):
                    if os.path.isdir(s):
                        shutil.copytree(s, dst)
                    else:
                        shutil.copy2(s, dst)
                    copied.append(item)

    if copied:
        public["input"]["files"] = ["assets/%s" % c for c in copied]

    # 输出规范（对被测 agent 可见）
    eo = task["expected_output"]
    if eo.get("type") == "json":
        public["output_spec"] = {"type": "json", "file": "result.json", "note": "输出合法 JSON，写入 result.json"}
    elif eo.get("type") == "text":
        public["output_spec"] = {"type": "text", "file": "result.txt", "note": "输出文本，写入 result.txt"}
    elif eo.get("type") == "code":
        public["output_spec"] = {"type": "code", "file": "solution.py", "note": "产出代码文件 solution.py"}
    else:
        public["output_spec"] = {"type": "file", "file": eo.get("file", "result.txt"), "note": "产出文件"}

    with open(os.path.join(out_public, tid, "task.json"), "w", encoding="utf-8") as fh:
        json.dump(public, fh, ensure_ascii=False, indent=2)
    return tid, public


def write_instructions(out_public, public_tasks):
    """生成 INSTRUCTIONS.md 总清单。"""
    lines = [
        "# 任务清单（Agent 执行说明）",
        "",
        "你正在接受一次标准化能力评测。请严格按照本说明执行，不要做任何说明之外的事。",
        "",
        "## 任务总数",
        "",
        "%d 个任务。每个任务在 tasks_public/<task_id>/ 下：task.json 是任务指令，assets/ 是输入素材。" % len(public_tasks),
        "",
        "## 执行步骤",
        "",
        "1. 依次读取 tasks_public/<task_id>/task.json；",
        "2. 在 outputs/<task_id>/ 目录下完成该任务；",
        "3. 按 task.json 的 output_spec 写出产物：",
        "   - JSON 任务 → outputs/<task_id>/result.json",
        "   - 文本任务 → outputs/<task_id>/result.txt",
        "   - 代码任务 → outputs/<task_id>/solution.py",
        "4. 不要修改任务文件，不要留下多余文件。",
        "",
        "## 任务清单",
        "",
        "| task_id | 类别 | 难度 | 标题 | 输出 |",
        "|---------|------|:---:|------|------|",
    ]
    for tid in sorted(public_tasks):
        t = public_tasks[tid]
        lines.append("| %s | %s | %s | %s | %s |" % (
            tid, t.get("category", ""), t.get("difficulty", ""), t.get("title", ""),
            t.get("output_spec", {}).get("file", "")))
    lines += [
        "",
        "## 红线（违反将判为不通过）",
        "",
        "- 禁止访问 benchmark 库内的原始任务目录（tasks/ 源文件）与任何 reference 文件；",
        "- 禁止修改 tasks_public/ 下的任务文件；",
        "- 禁止在输出中夹带与任务无关的内容；",
        "- 所有产物必须写入 outputs/ 目录。",
    ]
    with open(os.path.join(out_public, "INSTRUCTIONS.md"), "w", encoding="utf-8") as fh:
        fh.write("\n".join(lines))


def main():
    import argparse
    ap = argparse.ArgumentParser(description="导出被测 Agent 公开任务包（剥离参考/评分信息）")
    ap.add_argument("--benchmark", default="", help="benchmark 库根目录（默认从脚本位置向上探测）")
    ap.add_argument("--tasks", default="", help="可选，逗号分隔的 task_id 子集")
    ap.add_argument("--out", required=True, help="输出目录（将创建 tasks_public/）")
    args = ap.parse_args()

    root = os.path.abspath(args.benchmark) if args.benchmark else find_benchmark_root(SKILL_DIR)
    if not root:
        print("未找到 benchmark 库根（需含 tasks/ 与 schema/task.schema.json），请用 --benchmark 指定")
        sys.exit(2)
    print("Benchmark 库: %s" % root)

    all_tasks = collect_tasks(root)
    if args.tasks:
        ids = [t.strip() for t in args.tasks.split(",") if t.strip()]
        missing = [t for t in ids if t not in all_tasks]
        if missing:
            print("任务不存在: %s" % missing)
            sys.exit(2)
    else:
        ids = sorted(all_tasks.keys())

    out_public = os.path.join(args.out, "tasks_public")
    if os.path.exists(out_public):
        shutil.rmtree(out_public)
    os.makedirs(out_public, exist_ok=True)

    public_tasks = {}
    for tid in ids:
        public_tasks[tid] = export_task(all_tasks[tid], root, out_public)
        public_tasks[tid] = public_tasks[tid][1]
        print("  导出 %s: %s" % (tid, public_tasks[tid].get("title", "")))

    write_instructions(out_public, public_tasks)
    print("\n完成：任务包已导出到 %s" % out_public)
    print("下一步：被测 Agent 按 INSTRUCTIONS.md 执行，产物写入 outputs/<task_id>/")


if __name__ == "__main__":
    main()
