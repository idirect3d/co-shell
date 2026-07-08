// Author: L.Shuang
// Created: 2026-07-08
// Last Modified: 2026-07-08
//
// MIT License
// Copyright (c) 2026 L.Shuang
package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/docx"
)

// wordOpenTool opens a DOCX file and returns a session ID.
func (a *Agent) wordOpenTool(ctx context.Context, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path argument is required")
	}

	mode, _ := args["mode"].(string)
	if mode == "" {
		return "", fmt.Errorf("mode argument is required (create/read/copy)")
	}

	// Resolve relative paths
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot get current working directory: %w", err)
		}
		path = filepath.Join(cwd, path)
	}

	sessionID, err := a.docxSessionMgr.openWithMode(path, mode)
	if err != nil {
		return "", err
	}

	s, err := a.docxSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	fileInfo, _ := os.Stat(s.Path)
	fileSize := ""
	if fileInfo != nil {
		fileSize = fmt.Sprintf("%.1f KB", float64(fileInfo.Size())/1024.0)
	}
	if mode == "create" && fileSize == "0.0 KB" {
		fileSize = "(新建)"
	}

	var resultMsg string
	if mode == "copy" {
		resultMsg = fmt.Sprintf("已创建副本: %s (%s)\n会话 ID: %s\n\n此文件为 %s 的副本，后续操作应直接在此副本上修改，严禁以此文件替换原始文件。\n\n请使用 word_overview 获取文件概览。", s.Path, fileSize, sessionID, filepath.Base(path))
	} else if mode == "read" {
		resultMsg = fmt.Sprintf("已打开 Word 文件: %s (%s) (只读)\n会话 ID: %s\n\n请使用 word_overview 获取文件概览。", s.Path, fileSize, sessionID)
	} else {
		resultMsg = fmt.Sprintf("已打开 Word 文件: %s (%s)\n会话 ID: %s\n\n请使用 word_overview 获取文件概览。", s.Path, fileSize, sessionID)
	}
	return resultMsg, nil
}

// wordSaveTool saves a DOCX session without closing.
func (a *Agent) wordSaveTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	s, err := a.docxSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	if s.ReadOnly {
		return "", fmt.Errorf("此文件以只读方式打开，无法保存")
	}

	if err := a.docxSessionMgr.save(sessionID); err != nil {
		return "", err
	}

	fileInfo, _ := os.Stat(s.Path)
	fileSize := ""
	if fileInfo != nil {
		fileSize = fmt.Sprintf("%.1f KB", float64(fileInfo.Size())/1024.0)
	}

	return fmt.Sprintf("已保存: %s (%s)", s.Path, fileSize), nil
}

// wordCloseTool closes a DOCX session.
func (a *Agent) wordCloseTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	s, err := a.docxSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}
	path := s.Path

	if err := a.docxSessionMgr.close(sessionID); err != nil {
		return "", err
	}

	return fmt.Sprintf("已关闭 Word 文件: %s", path), nil
}

// wordOverviewTool returns an overview of the document structure.
func (a *Agent) wordOverviewTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	s, err := a.docxSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	return s.Doc.Overview(), nil
}

// wordReadTool reads paragraph range as HTML, text, or markdown.
func (a *Agent) wordReadTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	s, err := a.docxSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	fromPara := getIntArg(args, "from_para", 1) - 1
	toPara := getIntArg(args, "to_para", s.Doc.NumParagraphs()) - 1
	format, _ := args["format"].(string)
	if format == "" {
		format = "simple"
	}

	count, content, err := s.Doc.ReadParagraphRange(fromPara, toPara, format)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("段落 %d-%d (共 %d 段):\n%s", fromPara+1, toPara+1, count, content), nil
}

// wordTableReadTool reads a table as HTML.
func (a *Agent) wordTableReadTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	s, err := a.docxSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	tableIdx := getIntArg(args, "table_index", 0)
	format, _ := args["format"].(string)
	if format == "" {
		format = "simple"
	}

	// Find table by index
	tableCount := 0
	for _, elem := range s.Doc.Body {
		if elem.Kind == docx.ElemKindTable {
			if tableCount == tableIdx {
				return docx.TableToHTML(elem.Table, format), nil
			}
			tableCount++
		}
	}

	return "", fmt.Errorf("table index %d not found", tableIdx)
}

// wordContinueTool inserts content after/before a paragraph with format inheritance.
func (a *Agent) wordContinueTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	s, err := a.docxSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	content, _ := args["content"].(string)
	if content == "" {
		return "", fmt.Errorf("content argument is required")
	}

	afterPara := getIntArg(args, "after_para", 0)
	sameStyleAs := getIntArg(args, "same_style_as", 0)
	styleName, _ := args["style"].(string)

	// Determine reference style
	styleID := ""
	if styleName != "" {
		// Explicit style name
		styleDef := s.Doc.LookupStyle(styleName)
		if styleDef != nil {
			styleID = styleDef.ID
		}
	} else if sameStyleAs > 0 {
		refPara := s.Doc.GetParagraphByIndex(sameStyleAs - 1)
		if refPara != nil && refPara.StyleID != "" {
			styleID = refPara.StyleID
		}
	}

	// Insert after the specified paragraph
	insertedParas := parseContentAndInsert(s.Doc, afterPara, styleID, content)

	if len(insertedParas) == 0 {
		return "", fmt.Errorf("no content to insert")
	}

	s.Dirty = true

	var paraInfo []string
	for _, p := range insertedParas {
		paraInfo = append(paraInfo, fmt.Sprintf("#%d [%s] %s", p.Index+1, p.StyleName, truncateText(p.Text(), 60)))
	}

	return fmt.Sprintf("已插入 %d 个段落:\n%s\n(未保存，请调用 word_save 持久化)", len(insertedParas), strings.Join(paraInfo, "\n")), nil
}

// parseContentAndInsert parses Markdown-like content and inserts paragraphs.
func parseContentAndInsert(doc *docx.Document, afterPara int, defaultStyleID, content string) []*docx.Paragraph {
	var result []*docx.Paragraph

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		styleID := defaultStyleID
		text := trimmed

		// Detect heading markers
		if strings.HasPrefix(trimmed, "## ") {
			if styleID == "" {
				styleID = "Heading2"
			}
			text = strings.TrimPrefix(trimmed, "## ")
		} else if strings.HasPrefix(trimmed, "# ") {
			if styleID == "" {
				styleID = "Heading1"
			}
			text = strings.TrimPrefix(trimmed, "# ")
		} else if strings.HasPrefix(trimmed, "### ") {
			if styleID == "" {
				styleID = "Heading3"
			}
			text = strings.TrimPrefix(trimmed, "### ")
		} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			text = strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* ")
		}

		if styleID == "" {
			styleID = "Normal"
		}

		// Insert paragraph
		p := doc.AddParagraph(styleID, text)
		result = append(result, p)
	}

	return result
}

// wordEraseTool deletes paragraphs in a range.
func (a *Agent) wordEraseTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	s, err := a.docxSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	fromPara := getIntArg(args, "from_para", 1) - 1
	toPara := getIntArg(args, "to_para", 1) - 1

	if toPara < fromPara {
		return "", fmt.Errorf("to_para must be >= from_para")
	}

	deleted := s.Doc.RemoveParagraphRange(fromPara, toPara)
	s.Dirty = true

	return fmt.Sprintf("已删除 %d 个段落 (段落 #%d-#%d)\n(未保存，请调用 word_save 持久化)", deleted, fromPara+1, toPara+1), nil
}

// wordInspectStyleTool inspects a named style definition.
func (a *Agent) wordInspectStyleTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	s, err := a.docxSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name argument is required")
	}

	styleDef := s.Doc.LookupStyle(name)
	if styleDef == nil {
		return "", fmt.Errorf("style %q not found. Available styles: %s", name, strings.Join(s.Doc.StyleIDs(), ", "))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("样式: %s (ID: %s)\n", styleDef.Name, styleDef.ID))
	sb.WriteString(fmt.Sprintf("CSS: %s\n", styleDef.StyleCSS()))
	if styleDef.NextStyle != "" {
		sb.WriteString(fmt.Sprintf("后续样式: %s\n", styleDef.NextStyle))
	}

	sb.WriteString("\n用法: 请参考 word_overview 中的样式使用情况。")

	return sb.String(), nil
}

// wordFormatTool changes paragraph style or formatting.
func (a *Agent) wordFormatTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	s, err := a.docxSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	what, _ := args["what"].(string)
	value, _ := args["value"].(string)
	target, _ := args["target"].(string) // "style:Heading1" or "para:3-5"

	if what == "" || target == "" {
		return "", fmt.Errorf("what and target arguments are required")
	}

	affected := 0

	if strings.HasPrefix(target, "style:") {
		styleID := strings.TrimPrefix(target, "style:")
		for _, elem := range s.Doc.Body {
			if elem.Kind == docx.ElemKindParagraph && elem.Para != nil {
				if elem.Para.StyleID == styleID || elem.Para.StyleName == styleID {
					applyFormatChange(elem.Para, what, value)
					affected++
				}
			}
		}
	} else if strings.HasPrefix(target, "para:") {
		// Format: "para:3-5" or "para:3"
		rangeStr := strings.TrimPrefix(target, "para:")
		parts := strings.SplitN(rangeStr, "-", 2)
		fromPara, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return "", fmt.Errorf("invalid para range: %s", rangeStr)
		}
		toPara := fromPara
		if len(parts) > 1 {
			toPara, err = strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return "", fmt.Errorf("invalid para range: %s", rangeStr)
			}
		}
		for i := fromPara - 1; i <= toPara-1; i++ {
			p := s.Doc.GetParagraphByIndex(i)
			if p != nil {
				applyFormatChange(p, what, value)
				affected++
			}
		}
	}

	s.Dirty = true

	return fmt.Sprintf("已修改 %d 个段落的 %s\n(未保存，请调用 word_save 持久化)", affected, what), nil
}

// applyFormatChange applies a single format change to a paragraph.
func applyFormatChange(p *docx.Paragraph, what, value string) {
	switch what {
	case "style":
		if value != "" {
			p.StyleID = value
		}
	case "font_name":
		for _, r := range p.Runs {
			if r.RPr == nil {
				r.RPr = &docx.RunProps{}
			}
			r.RPr.FontName = value
		}
	case "font_size":
		if sz, err := strconv.ParseFloat(value, 64); err == nil {
			for _, r := range p.Runs {
				if r.RPr == nil {
					r.RPr = &docx.RunProps{}
				}
				r.RPr.FontSize = sz
			}
		}
	case "bold":
		for _, r := range p.Runs {
			if r.RPr == nil {
				r.RPr = &docx.RunProps{}
			}
			r.RPr.Bold = value == "true" || value == "1"
		}
	case "italic":
		for _, r := range p.Runs {
			if r.RPr == nil {
				r.RPr = &docx.RunProps{}
			}
			r.RPr.Italic = value == "true" || value == "1"
		}
	case "color":
		for _, r := range p.Runs {
			if r.RPr == nil {
				r.RPr = &docx.RunProps{}
			}
			r.RPr.Color = value
		}
	}
}

// wordStyleCloneTool clones format from a paragraph to create a new named style.
func (a *Agent) wordStyleCloneTool(ctx context.Context, args map[string]interface{}) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id argument is required")
	}

	s, err := a.docxSessionMgr.get(sessionID)
	if err != nil {
		return "", err
	}

	fromPara := getIntArg(args, "from_para", 1) - 1
	newName, _ := args["new_style_name"].(string)

	if newName == "" {
		return "", fmt.Errorf("new_style_name is required")
	}

	p := s.Doc.GetParagraphByIndex(fromPara)
	if p == nil {
		return "", fmt.Errorf("paragraph %d not found", fromPara+1)
	}

	// Create a new style entry
	newStyle := &docx.StyleDefinition{
		ID:   strings.ReplaceAll(newName, " ", ""),
		Name: newName,
		Type: "paragraph",
	}

	// Copy from existing style
	if existing, ok := s.Doc.Styles[p.StyleID]; ok && existing.RPr != nil {
		rp := *existing.RPr
		newStyle.RPr = &rp
	}

	// Copy paragraph format
	if p.PPr != nil {
		pp := *p.PPr
		newStyle.PPr = &pp
	}

	s.Doc.Styles[newStyle.ID] = newStyle
	s.Dirty = true

	return fmt.Sprintf("已创建样式 %q (ID: %s)\nCSS: %s\n(未保存，请调用 word_save 持久化)", newName, newStyle.ID, newStyle.StyleCSS()), nil
}

// truncateText truncates text to maxLen characters.
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
