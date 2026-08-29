#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""AgentBenchmark Skill — 离线评分脚本。

读取被测 Agent 在 outputs/<task_id>/ 下的产物，按 benchmark 库原始任务定义
（含隐藏的 reference）评分，聚合生成评测报告。

用法:
    python scripts/score_outputs.py --benchmark ../../ --outputs run_x/outputs --run-id run_x
    python scripts/score_outputs.py --benchmark ../../ --outputs run_x/outputs --judge-api ... --judge-model ...

产物约定（被测 Agent 按 INSTRUCTIONS.md 执行）:
    outputs/<task_id>/result.json    # JSON 任务
    outputs/<task_id>/result.txt     # 文本任务
    outputs/<task_id>/solution.py    # 代码任务

说明:
    - 本脚本必须由"评测方"运行（而非被测 Agent 自评），以保证公平；
    - reference / scoring 从库内原始 task.json 读取，被测 Agent 全程不可见。
"""
import datetime
import json
import os
import sys

SKILL_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def find_benchmark_root(start):
    cur = os.path.abspath(start)
    while True:
        if os.path.isdir(os.path.join(cur, "tasks")) and os.path.isfile(os.path.join(cur, "schema", "task.schema.json")):
            return cur
        parent = os.path.dirname(cur)
        if parent == cur:
            return None
        cur = parent


def load_task(root, task_id):
    for dirpath, _, files in os.walk(os.path.join(root, "tasks")):
        for f in files:
            if f.endswith(".task.json"):
                p = os.path.join(dirpath, f)
                with open(p, "r", encoding="utf-8") as fh:
                    t = json.load(fh)
                if t["task_id"] == task_id:
                    return t
    return None


def collect_outputs(outputs_dir):
    """返回 {task_id: 目录路径}（outputs/ 下的子目录）。"""
    found = {}
    if not os.path.isdir(outputs_dir):
        return found
    for name in sorted(os.listdir(outputs_dir)):
        p = os.path.join(outputs_dir, name)
        if os.path.isdir(p):
            found[name] = p
    return found


def resolve_actual(task, task_dir):
    """从产物目录解析出评分器可用的 actual（参考 harness.collect_output）。"""
    eo = task["expected_output"]
    etype = eo.get("type", "text")
    if etype == "json":
        for fname in ("result.json", "result.txt"):
            p = os.path.join(task_dir, fname)
            if os.path.exists(p):
                with open(p, "r", encoding="utf-8") as fh:
                    text = fh.read()
                try:
                    return json.loads(text)
                except (json.JSONDecodeError, ValueError):
                    return text
        return None
    if etype == "text":
        p = os.path.join(task_dir, "result.txt")
        if os.path.exists(p):
            with open(p, "r", encoding="utf-8") as fh:
                return fh.read()
        return None
    if etype == "code":
        return None  # actual 为 None，由 code_test 评分器在 sandbox=task_dir 中运行测试
    # file 类型
    target = eo.get("file", "result.txt")
    p = os.path.join(task_dir, os.path.basename(target))
    if not os.path.exists(p):
        return None
    with open(p, "r", encoding="utf-8") as fh:
        return fh.read()


def main():
    sys.path.insert(0, SKILL_DIR)
    sys.path.insert(0, os.path.dirname(os.path.dirname(SKILL_DIR)))  # benchmark 库根（含 runner/scorers）
    from runner.harness import copy_test_files  # noqa: E402
    from scorers.base import run_scorer  # noqa: E402
    from metrics.metrics import aggregate  # noqa: E402
    from report import report as report_mod  # noqa: E402

    import argparse
    ap = argparse.ArgumentParser(description="离线评分：读取被测 Agent 产物并生成报告")
    ap.add_argument("--benchmark", default="", help="benchmark 库根目录（默认向上探测）")
    ap.add_argument("--outputs", required=True, help="被测 Agent 产物根目录（含 <task_id>/ 子目录）")
    ap.add_argument("--run-id", default="", help="报告命名（默认时间戳）")
    ap.add_argument("--judge-api", default="")
    ap.add_argument("--judge-model", default="")
    ap.add_argument("--judge-key", default="")
    args = ap.parse_args()

    root = os.path.abspath(args.benchmark) if args.benchmark else find_benchmark_root(SKILL_DIR)
    if not root:
        print("未找到 benchmark 库根，请用 --benchmark 指定")
        sys.exit(2)
    print("Benchmark 库: %s" % root)
    print("产物目录: %s" % args.outputs)

    judge = None
    if args.judge_api and args.judge_model:
        judge = {"api": args.judge_api, "model": args.judge_model, "api_key": args.judge_key}

    out_dirs = collect_outputs(args.outputs)
    if not out_dirs:
        print("产物目录为空或不存在: %s" % args.outputs)
        sys.exit(2)

    results = []
    missing_meta = []
    for tid, task_dir in out_dirs.items():
        task = load_task(root, tid)
        if task is None:
            missing_meta.append(tid)
            continue
        actual = resolve_actual(task, task_dir)
        ctx = {"sandbox": task_dir, "judge": judge}
        copy_test_files(task, task_dir, root)  # 把测试文件拷入产物目录（供 code_test 运行）
        sr = run_scorer(task, actual, ctx)
        results.append({
            "task_id": tid,
            "title": task.get("title", ""),
            "category": task.get("category", ""),
            "difficulty": task.get("difficulty", 1),
            "version": task.get("version", ""),
            "score": sr.score,
            "passed": sr.passed,
            "threshold": task["scoring"].get("pass_threshold", 0.8),
            "details": sr.details,
            "elapsed": 0.0,
            "steps": 0,
            "tokens": 0,
            "safety": {"violations": [], "leak_detected": False, "score": 1.0},
        })
        print("  %s %s score=%.4f %s" % ("✅" if sr.passed else "❌", tid, sr.score, sr.passed))

    if missing_meta:
        print("警告：以下 task_id 在库中无对应任务定义（跳过评分）: %s" % missing_meta)
    if not results:
        print("无有效产物可评分")
        sys.exit(2)

    agg = aggregate(results)
    run_id = args.run_id or datetime.datetime.now().strftime("run_%Y%m%d_%H%M%S")
    out_root = os.path.dirname(os.path.abspath(args.outputs))
    run_dir = os.path.join(out_root, "score_%s" % run_id)
    meta = {
        "run_id": run_id,
        "agent": "offline-scored",
        "timestamp": datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        "judge_model": args.judge_model or "heuristic",
        "outputs_dir": os.path.abspath(args.outputs),
    }
    md_path = report_mod.write_results(run_dir, meta, results, agg)

    print("\n==== 评分汇总 ====")
    print("任务数: %d ｜ 通过率: %.1f%% (%d/%d)" % (agg["count"], agg["pass_rate"] * 100, agg["passed"], agg["count"]))
    print("平均任务分: %.4f ｜ 按类别: %s" % (agg["avg_score"], json.dumps(agg["by_category"], ensure_ascii=False)))
    print("按难度: %s" % json.dumps(agg["by_difficulty"], ensure_ascii=False))
    print("\n报告: %s" % md_path)


if __name__ == "__main__":
    main()
