#!/usr/bin/env python3
"""Add VisualAnalysisMaxImages entries to i18n/en.go."""
import sys

filepath = sys.argv[1]
with open(filepath, 'r') as f:
    content = f.read()

# Add KeySettingsDescVisualAnalysisMaxImages after KeySettingsDescRepetitionPenalty
old1 = 'KeySettingsDescRepetitionPenalty: "Repetition penalty (0.0 ~ 2.0, -1 = don\'t send)",'
new1 = old1 + '\n\tKeySettingsDescVisualAnalysisMaxImages: "Max images per visual_analysis call (1-20, default 5)",'
content = content.replace(old1, new1)

# Add KeyCol3VisualAnalysisMaxImages after KeyCol3RepetitionPenalty
old2 = '\tKeyCol3RepetitionPenalty: "Repetition penalty parameter",'
new2 = old2 + '\n\tKeyCol3VisualAnalysisMaxImages: "Visual analysis max images (1-20)",'
content = content.replace(old2, new2)

with open(filepath, 'w') as f:
    f.write(content)
print("Done")