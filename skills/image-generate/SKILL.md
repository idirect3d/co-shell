---
name: image-generate
description: >-
  本地 AI 文生图（画图）能力。当用户提出"生成图片 / 画图 / 绘制插图 / 文生图 /
  AI 绘画 / 配图 / 海报 / 示意图"等请求时使用。基于 Qwen-Image-Edit-2511
  （mflux + MLX 本地推理），输入英文提示词即可生成 PNG 图片，支持自定义尺寸、
  步数、CFG、随机种子。全程本地运行，无需联网、无需 API Key。
---

# 文生图（image-generate）技能

## 目标

引导当前 Agent 使用**本地 AI 图像生成**能力：根据用户描述（提示词）生成图片，
产出 PNG 文件并交付给用户。基于 `mflux-generate-qwen`（Qwen-Image-Edit-2511，
MLX 8bit 量化，Apple Silicon 本地推理）。

## 环境依赖（必须先确认）

1. **Python 环境**：`~/.venv-vllm-metal`（含 mflux），入口 `~/.venv-vllm-metal/bin/mflux-generate-qwen`
2. **模型**：`mlx-community/qwen-image-edit-2511-8bit`（已缓存在 `~/.cache/huggingface/hub/`，
   无需联网下载；若缓存缺失需经 hf-mirror 下载）
3. **硬件**：Apple Silicon（M 系列），生成 1280×720@24 步约 3 分钟、峰值内存 ~37GB；
   内存 < 32GB 时建议减小尺寸或加 `--low-ram`

## 执行流程

### 阶段 1：确认需求

- 明确用户要什么图：主体、风格（卡通/写实/扁平/水墨等）、画幅（横版/竖版/方形）、用途
- 若用户描述模糊，先用一句话复述确认，再生成

### 阶段 2：构造提示词（关键）

- **英文提示词效果最佳**（模型对英文理解与文字渲染更强）
- 结构建议：主体 → 细节 → 风格 → 背景 → 画质词
- 示例：`a cute robot wearing a seashell helmet, flat vector illustration, soft pastel colors, clean white background, high quality`
- 需要画面内文字时：只放**少量英文短标签**（如标题、模块名），中文/长句易乱码
- 若需精确文字排版图（如带大量文字的架构图），提示模型改用绘图工具（HTML/SVG）而非本技能

### 阶段 3：生成

```bash
python <skill>/scripts/generate_image.py \
  --prompt "英文提示词" \
  --output outputs/<名称>.png \
  [--width 1280] [--height 720] \
  [--steps 20] [--guidance 4.5] [--seed 42]
```

参数说明（默认值见脚本）：

| 参数 | 说明 | 默认 |
|------|------|------|
| `-p/--prompt` | 英文提示词（必填） | - |
| `-o/--output` | 输出 PNG 路径（必填） | - |
| `-w/--width` | 宽（建议 ≤1280，须为 16 的倍数） | 1024 |
| `-H/--height` | 高（建议 ≤1280，须为 16 的倍数） | 1024 |
| `-s/--steps` | 推理步数（越高越精细越慢） | 20 |
| `-g/--guidance` | CFG 强度（3.5~6） | 4.5 |
| `-r/--seed` | 随机种子（同种子可复现） | 42 |
| `-np/--negative-prompt` | 负向提示词（可选） | 内置默认 |

### 阶段 4：交付

- 用 `open <输出路径>` 在用户屏幕展示图片（macOS）
- 告知用户：输出路径、尺寸、生成耗时
- 如需调整：改提示词/尺寸/种子重新生成（同种子不同提示词 → 不同图）

## 常见问题

| 问题 | 处理 |
|------|------|
| `mflux-generate-qwen: command not found` | 确认 `~/.venv-vllm-metal/bin` 存在且 mflux 已安装 |
| 模型下载很慢 | 首次缺模型时脚本已设 `HF_ENDPOINT=https://hf-mirror.com` 镜像 |
| 图片文字乱码 | 属模型已知局限，改用英文短标签或改用 HTML/SVG 绘图 |
| 内存不足（OOM） | 降低宽高（如 768×768）或减小 steps |
| 效果不理想 | 调整 seed 重跑（换一个随机结果），或优化提示词风格词 |

## 红线

- ❌ 不生成违法、违规、侵犯他人权益的内容（肖像、商标、敏感内容）
- ❌ 不承诺"图片文字 100% 正确"（模型文字渲染有局限）
- ✅ 输出必须落盘为 PNG 并给出路径，不得只描述"想象出来的图"
