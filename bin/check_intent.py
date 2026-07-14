#!/usr/bin/env python3
"""
检查 i18n 系统提示词中哪些工具的 XML 用法示例缺少 <intent> 参数。
"""
import re

# 不要求 intent 的工具
SKIP = {
    'track_task_progress', 'attempt_completion', 'ask_followup_question',
    'reorganize_context', 'view_task_plan',
    'evaluate_expression', 'add_images', 'shell_reset',
}

def check_file(filepath):
    with open(filepath, 'r') as f:
        content = f.read()
    
    # 找到所有 ## tool_name 节（只匹配有效的工具名，排除示例中的子标题）
    tools = re.findall(r'^## (excel_\w+|word_\w+|vault_\w+|browser_\w+|shell_\w+|execute_command|read_file|search_files|list_files|list_code_definition_names|replace_in_file|write_to_file|visual_analysis|launch_sub_agent|schedule_task|get_memory_slice|memory_search|delete_memory|update_settings|list_settings)$', content, re.MULTILINE)
    
    missing = []
    for tool in tools:
        if tool in SKIP:
            continue
        idx = content.index(f'## {tool}')
        end = content.find('\n## ', idx+1)
        if end == -1:
            end = len(content)
        section = content[idx:end]
        if '<intent>' not in section:
            missing.append(tool)
    
    return missing

for lang in ['en_system.go', 'zh_system.go']:
    path = f'i18n/{lang}'
    missing = check_file(path)
    print(f'=== {lang} ===')
    if missing:
        print(f'缺失 <intent> 的工具 ({len(missing)}):')
        for t in missing:
            print(f'  - {t}')
    else:
        print('全部包含 <intent>')
    print()