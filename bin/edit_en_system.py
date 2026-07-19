#!/usr/bin/env python3
"""Update visual_analysis tool description in i18n/en_system.go."""
import sys

filepath = sys.argv[1]
with open(filepath, 'r', encoding='utf-8') as f:
    content = f.read()

old = r'enMessages[KeyToolUsageVisualAnalysis] = `## visual_analysis
Description: Load one visual media file (image, screenshot, scanned document, video frame, etc.) for multimodal vision analysis. Provide a single file path and specify what to analyze. The file is sent to the LLM exactly once and automatically removed from cache after delivery. Supports: OCR/text recognition, image understanding, table/data extraction, document analysis, video frame analysis, etc. To analyze multiple files, call this tool once per file. **You MUST specify the \'intent\' parameter to describe what specific information to analyze.**
Parameters:
- path (required) Single image/video file path to load for visual analysis (e.g., \'screenshot.png\', \'diagram.jpg\', \'video_frame.mp4\')
- intent (required) Describe what specific information to analyze from the image/video. Examples: \'Extract invoice amounts and dates\', \'Extract all data columns from the table\', \'Describe the scene and people in this photo\', \'Analyze the code errors shown in the screenshot\'
Usage:
<visual_analysis>
  <path>screenshot.png</path>
  <intent>Extract the invoice amount and date from this image</intent>
</visual_analysis>`'

new = r'enMessages[KeyToolUsageVisualAnalysis] = `## visual_analysis
Description: Load one or more visual media files (images, screenshots, scanned documents, video frames, etc.) for multimodal vision analysis. Provide an array of file paths and specify what to analyze. The files are sent to the LLM exactly once and automatically removed from cache after delivery. Supports: OCR/text recognition, image understanding, table/data extraction, document analysis, video frame analysis, etc. The maximum number of files per call is controlled by the visual-analysis-max-images config setting (default: 5). **You MUST specify the \'intent\' parameter to describe what specific information to analyze.**
Parameters:
- paths (required) Array of image/video file paths to load for visual analysis. Example: [\'page1.png\', \'page2.png\', \'diagram.jpg\']
- intent (required) Describe what specific information to analyze from the image/video. Examples: \'Extract invoice amounts and dates\', \'Extract all data columns from the table\', \'Describe the scene and people in this photo\', \'Analyze the code errors shown in the screenshot\'
Usage:
<visual_analysis>
  <paths>
    <item>screenshot1.png</item>
    <item>screenshot2.png</item>
  </paths>
  <intent>Extract the key information from these screenshots</intent>
</visual_analysis>`'

if old in content:
    content = content.replace(old, new)
    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(content)
    print("Updated en_system.go")
else:
    print("ERROR: Could not find old content in en_system.go")
    sys.exit(1)