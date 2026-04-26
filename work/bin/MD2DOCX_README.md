# md2docx — Markdown 转 Word 转换器

**位置:** `workspace/bin/md2docx.py`
**别名:** `~/bin/md2docx` (已加入 PATH)

## 快速使用

```bash
# 最简用法（自动生成同名 .docx）
md2docx 文档.md

# 指定输出路径
md2docx 文档.md -o 输出路径/文档.docx

# 完整路径调用
python3 workspace/tool/md2docx.py 文档.md
```

## 所有参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `input` | 输入的 Markdown 文件路径 | **(必填)** |
| `-o, --output` | 输出的 Word 文件路径 | 输入文件名 + .docx |
| `--style` | 视觉风格: modern / classic / minimal / **official** | `modern` |
| `--title` | 文档标题（封面用） | 文件名 |
| `--author` | 作者名（封面用） | `co-shell` |
| `--font-size` | 正文字号 (pt) | `11` |
| `--font-family` | 字体名称 | `Arial` |
| `--no-toc` | 跳过目录生成 | — |
| `--no-cover` | 跳过封面页 | — |
| `--debug` | 打印调试信息 | — |

## 公文格式（--style official）

按《党政机关公文格式》（GB/T 9704-2012）设置：

| 项目 | 规范值 |
|------|--------|
| 纸张 | A4 (210mm × 297mm) |
| 上边距 | 37mm |
| 下边距 | 35mm |
| 左边距 | 28mm |
| 右边距 | 26mm |
| 正文 | 仿宋_GB2312，三号 (16pt) |
| 行距 | 固定值 28磅 |
| 首行缩进 | 2字符 |
| 一级标题 (#) | 黑体，二号 (22pt)，居中 |
| 二级标题 (##) | 黑体，三号 (16pt) |
| 三级标题 (###) | 楷体_GB2312，三号 (16pt) |

```bash
# 公文格式转换
md2docx 报告.md --style official

# 公文格式，自定义标题
md2docx 报告.md --style official --title "关于XXX的报告"
```

## 示例

```bash
# 经典风格，自定义标题和作者
md2docx README.md --style classic --title "项目文档" --author "团队"

# 极简风格，不要封面和目录
md2docx README.md --style minimal --no-cover --no-toc

# 公文格式
md2docx README.md --style official

# 调试模式查看详细信息
md2docx README.md --debug

# 自定义字体
md2docx README.md --font-size 12 --font-family "Helvetica"
```

## 支持格式

- **标题** `# ~ ######` (自动生成目录)
- **粗体** `**文字**`
- **斜体** `*文字*`
- **代码** `` `代码` ``
- **链接** `[文字](url)`
- **代码块** ` ``` ` (带背景色)
- **引用块** `>` (带左边框)
- **无序列表** `-` / `*` / `+`
- **有序列表** `1.` / `2.`
- **表格** `| 列1 | 列2 |`
- **图片** `![描述](路径)`
- **分割线** `---` / `***`
