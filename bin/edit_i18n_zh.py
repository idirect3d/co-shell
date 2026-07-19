#!/usr/bin/env python3
"""Add VisualAnalysisMaxImages entries to i18n/zh.go."""
import sys

filepath = sys.argv[1]
with open(filepath, 'r') as f:
    content = f.read()

# Add KeySettingsDescVisualAnalysisMaxImages after KeySettingsDescRepetitionPenalty
old1 = 'KeySettingsDescRepetitionPenalty: "重复惩罚参数（0.0 ~ 2.0，-1 不发送）",'
new1 = old1 + '\n\tKeySettingsDescVisualAnalysisMaxImages: "视觉分析单次调用可加载的最大图片数量(1-20)",'
content = content.replace(old1, new1)

# Add KeyCol3VisualAnalysisMaxImages after KeyCol3RepetitionPenalty
old2 = '\tKeyCol3RepetitionPenalty: "重复惩罚参数",'
new2 = old2 + '\n\tKeyCol3VisualAnalysisMaxImages: "视觉分析最大图片数(1-20)",'
content = content.replace(old2, new2)

with open(filepath, 'w') as f:
    f.write(content)
print("Done")