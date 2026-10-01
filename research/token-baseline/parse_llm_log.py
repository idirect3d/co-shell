#!/usr/bin/env python3
"""Parse co-shell llm-interaction logs into a per-call CSV (FEATURE-577 stage S6).

Each log record is written as:

    [2026-10-02 00:38:31.947] [REQ]
    { ... request JSON (messages, tools) ... }
    [2026-10-02 00:38:34.120] [RES]
    { ... response JSON (usage) ... }

The script pairs REQ/RES records inside an optional time window and reports, per
call: message count, <environment_details> occurrences, system prompt bytes,
tool declaration bytes, prompt/completion/cached tokens and cache hit/miss.

Usage:
  python3 parse_llm_log.py --log log/llm-interaction-2026-10-02.log \
      --from "2026-10-02 02:30:00" --to "2026-10-02 02:45:00" --out run-1
"""

import argparse
import csv
import json
import re

MARK = re.compile(r"^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3})\] \[(REQ|RESP)\]", re.M)
USAGE_KEYS = [
    "prompt_tokens",
    "completion_tokens",
    "cached_tokens",
    "prompt_cache_hit_tokens",
    "prompt_cache_miss_tokens",
    "total_tokens",
]


def iter_records(text):
    marks = list(MARK.finditer(text))
    for i, m in enumerate(marks):
        end = marks[i + 1].start() if i + 1 < len(marks) else len(text)
        yield m.group(1), m.group(2), text[m.end():end]


def find_usage(obj):
    if isinstance(obj, dict):
        if "prompt_tokens" in obj:
            return obj
        for v in obj.values():
            got = find_usage(v)
            if got:
                return got
    elif isinstance(obj, list):
        for v in obj:
            got = find_usage(v)
            if got:
                return got
    return None


def regex_usage(body):
    out = {}
    for k in USAGE_KEYS:
        m = re.search(r'"%s":\s*(\d+)' % k, body)
        if m:
            out[k] = int(m.group(1))
    return out or None


def parse_request(body, call):
    try:
        req = json.loads(body)
    except ValueError:
        return
    if not isinstance(req, dict):
        return
    msgs = req.get("messages") or []
    call["messages"] = len(msgs)
    if msgs and isinstance(msgs[0].get("content"), str):
        call["system_bytes"] = len(msgs[0]["content"])
    blob = json.dumps(msgs, ensure_ascii=False)
    call["env_count"] = blob.count("<environment_details>")
    tools = req.get("tools") or []
    call["tools"] = len(tools)
    if tools:
        call["tools_bytes"] = len(json.dumps(tools, ensure_ascii=False))


def parse_response(body, call):
    usage = None
    try:
        usage = find_usage(json.loads(body))
    except ValueError:
        usage = None
    if usage is None:
        usage = regex_usage(body)
    if not usage:
        return
    call["prompt"] = usage.get("prompt_tokens", 0)
    call["completion"] = usage.get("completion_tokens", 0)
    call["cached"] = usage.get("cached_tokens", 0)
    call["hit"] = usage.get("prompt_cache_hit_tokens", 0)
    call["miss"] = usage.get("prompt_cache_miss_tokens", 0)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--log", required=True)
    ap.add_argument("--out", required=True, help="output prefix, e.g. research/token-baseline/run-1")
    ap.add_argument("--from", dest="ts_from", default="")
    ap.add_argument("--to", dest="ts_to", default="")
    args = ap.parse_args()

    with open(args.log, encoding="utf-8", errors="replace") as fh:
        text = fh.read()

    calls, cur = [], None
    for ts, kind, body in iter_records(text):
        if args.ts_from and ts < args.ts_from:
            continue
        if args.ts_to and ts > args.ts_to:
            continue
        if kind == "REQ":
            if cur:
                calls.append(cur)
            cur = {"ts": ts, "messages": 0, "env_count": 0, "system_bytes": 0,
                   "tools": 0, "tools_bytes": 0, "prompt": 0, "completion": 0,
                   "cached": 0, "hit": 0, "miss": 0}
            parse_request(body, cur)
        elif cur is not None:
            parse_response(body, cur)
    if cur:
        calls.append(cur)

    fields = ["idx", "ts", "messages", "env_count", "system_bytes", "tools",
              "tools_bytes", "prompt", "completion", "cached", "hit", "miss"]
    with open(args.out + ".calls.csv", "w", newline="", encoding="utf-8") as fh:
        w = csv.writer(fh)
        w.writerow(fields)
        for i, c in enumerate(calls, 1):
            c["idx"] = i
            w.writerow([c[k] for k in fields])

    total_prompt = sum(c["prompt"] for c in calls)
    total_completion = sum(c["completion"] for c in calls)
    total_cached = sum(c["cached"] for c in calls)
    total_hit = sum(c["hit"] for c in calls)
    total_miss = sum(c["miss"] for c in calls)
    both = total_hit + total_miss
    rate = (100.0 * total_hit / both) if both else 0.0
    deltas = [calls[i]["prompt"] - calls[i - 1]["prompt"] for i in range(1, len(calls))]
    deltas.sort()
    median_delta = deltas[len(deltas) // 2] if deltas else 0
    with open(args.out + ".summary.txt", "w", encoding="utf-8") as fh:
        fh.write("rounds=%d\n" % len(calls))
        fh.write("total_prompt=%d\n" % total_prompt)
        fh.write("total_completion=%d\n" % total_completion)
        fh.write("total_cached=%d\n" % total_cached)
        fh.write("cache_hit_tokens=%d\n" % total_hit)
        fh.write("cache_miss_tokens=%d\n" % total_miss)
        fh.write("cache_hit_rate=%.1f%%\n" % rate)
        if calls:
            fh.write("first_prompt=%d\n" % calls[0]["prompt"])
            fh.write("last_prompt=%d\n" % calls[-1]["prompt"])
            fh.write("median_prompt_delta=%d\n" % median_delta)
            fh.write("tools=%d\n" % calls[0]["tools"])
            fh.write("tools_bytes=%d\n" % calls[0]["tools_bytes"])
            fh.write("system_bytes=%d\n" % calls[0]["system_bytes"])
            fh.write("messages_first=%d\n" % calls[0]["messages"])
            fh.write("messages_last=%d\n" % calls[-1]["messages"])
            fh.write("env_first=%d\n" % calls[0]["env_count"])
            fh.write("env_last=%d\n" % calls[-1]["env_count"])
    print("%s: rounds=%d total_prompt=%d cache_hit_rate=%.1f%%" %
          (args.out, len(calls), total_prompt, rate))


if __name__ == "__main__":
    main()
