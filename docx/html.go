// Author: L.Shuang
// Created: 2026-07-08
// Last Modified: 2026-07-08
//
// MIT License
// Copyright (c) 2026 L.Shuang
package docx

import (
	"fmt"
	"html"
	"strings"
)

// ParaToHTML converts a paragraph to an HTML string.
// format can be "simple" (structural only) or "full" (with inline style).
// Returns the HTML with a "N| " prefix (1-based index).
func ParaToHTML(p *Paragraph, format string) string {
	text := html.EscapeString(p.Text())
	if text == "" {
		return ""
	}

	element := styleToElementName(p.StyleName)
	prefix := fmt.Sprintf("%d| ", p.Index+1)

	if format == "full" {
		styleAttr := buildParaHTMLStyle(p)
		styleIDAttr := ""
		if p.StyleID != "" {
			styleIDAttr = fmt.Sprintf(` data-style-id="%s"`, html.EscapeString(p.StyleID))
		}
		inner := fmt.Sprintf(`<%s%s%s>%s</%s>`, element, styleIDAttr, styleAttr, text, element)
		return prefix + inner + "\n"
	}

	// simple format
	var inner string
	if p.StyleID != "" {
		inner = fmt.Sprintf(`<%s data-style="%s">%s</%s>`, element, html.EscapeString(p.StyleName), text, element)
	} else {
		inner = fmt.Sprintf(`<%s>%s</%s>`, element, text, element)
	}
	return prefix + inner + "\n"
}

// ParaToText converts a paragraph to plain text (no markup).
func ParaToText(p *Paragraph) string {
	text := p.Text()
	if text == "" {
		return ""
	}
	return fmt.Sprintf("%d| %s", p.Index+1, text)
}

// ParaToMarkdown converts a paragraph to Markdown format.
func ParaToMarkdown(p *Paragraph) string {
	text := p.Text()
	if text == "" {
		return ""
	}
	prefix := fmt.Sprintf("%d| ", p.Index+1)

	mdText := text
	styleName := strings.ToLower(p.StyleName)

	// Apply bold/italic from runs
	if len(p.Runs) > 0 {
		var mdParts []string
		for _, r := range p.Runs {
			t := r.Text
			if t == "" {
				continue
			}
			if r.RPr != nil {
				if r.RPr.Bold && r.RPr.Italic {
					t = "***" + t + "***"
				} else if r.RPr.Bold {
					t = "**" + t + "**"
				} else if r.RPr.Italic {
					t = "*" + t + "*"
				}
			}
			mdParts = append(mdParts, t)
		}
		if len(mdParts) > 0 {
			mdText = strings.Join(mdParts, "")
		}
	}

	if strings.Contains(styleName, "heading") || strings.Contains(styleName, "标题") {
		level := 1
		for _, part := range strings.Fields(styleName) {
			if len(part) == 1 && part[0] >= '1' && part[0] <= '6' {
				level = int(part[0] - '0')
				break
			}
		}
		marker := strings.Repeat("#", level)
		return fmt.Sprintf("%s %s %s", prefix, marker, mdText)
	}

	if strings.Contains(styleName, "list") || strings.Contains(styleName, "bullet") || strings.Contains(styleName, "列表") {
		return fmt.Sprintf("%s - %s", prefix, mdText)
	}

	if strings.Contains(styleName, "quote") || strings.Contains(styleName, "引言") {
		return fmt.Sprintf("%s > %s", prefix, mdText)
	}

	return fmt.Sprintf("%s %s", prefix, mdText)
}

// styleToElementName maps a paragraph style name to an HTML element.
func styleToElementName(styleName string) string {
	name := strings.ToLower(styleName)

	if strings.Contains(name, "heading") || strings.Contains(name, "标题") {
		for _, part := range strings.Fields(name) {
			if len(part) == 1 && part[0] >= '1' && part[0] <= '6' {
				return fmt.Sprintf("h%c", part[0])
			}
		}
		return "h1"
	}

	if strings.Contains(name, "list") || strings.Contains(name, "bullet") || strings.Contains(name, "列表") {
		return "li"
	}
	if strings.Contains(name, "quote") || strings.Contains(name, "引言") {
		return "blockquote"
	}
	if strings.Contains(name, "code") {
		return "pre"
	}

	return "p"
}

// buildParaHTMLStyle constructs a CSS style string from paragraph and run formatting.
func buildParaHTMLStyle(p *Paragraph) string {
	var parts []string

	if p.PPr != nil {
		if p.PPr.Alignment != "" {
			parts = append(parts, fmt.Sprintf("text-align:%s", p.PPr.Alignment))
		}
		if p.PPr.SpaceBefore > 0 {
			parts = append(parts, fmt.Sprintf("margin-top:%.1fpt", p.PPr.SpaceBefore))
		}
		if p.PPr.SpaceAfter > 0 {
			parts = append(parts, fmt.Sprintf("margin-bottom:%.1fpt", p.PPr.SpaceAfter))
		}
		if p.PPr.LineSpacing > 0 {
			parts = append(parts, fmt.Sprintf("line-height:%.1f", p.PPr.LineSpacing))
		}
		if p.PPr.FirstLineIndent > 0 {
			parts = append(parts, fmt.Sprintf("text-indent:%.1fpt", p.PPr.FirstLineIndent))
		}
	}

	if len(p.Runs) == 1 && p.Runs[0].RPr != nil {
		rp := p.Runs[0].RPr
		if rp.Bold {
			parts = append(parts, "font-weight:bold")
		}
		if rp.Italic {
			parts = append(parts, "font-style:italic")
		}
		if rp.FontSize > 0 {
			parts = append(parts, fmt.Sprintf("font-size:%.1fpt", rp.FontSize))
		}
		if rp.FontName != "" {
			parts = append(parts, fmt.Sprintf("font-family:%s", rp.FontName))
		}
		if rp.Color != "" {
			parts = append(parts, fmt.Sprintf("color:%s", rp.Color))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf(` style="%s"`, strings.Join(parts, "; "))
}

// TableToHTML converts a table to an HTML <table> string.
func TableToHTML(t *Table, format string) string {
	if len(t.Rows) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<table>\n")

	for _, row := range t.Rows {
		sb.WriteString("  <tr>")
		for _, cell := range row.Cells {
			cellText := html.EscapeString(cell.Text)

			attrs := ""
			if cell.Colspan > 1 {
				attrs += fmt.Sprintf(` colspan="%d"`, cell.Colspan)
			}
			if cell.Rowspan > 1 {
				attrs += fmt.Sprintf(` rowspan="%d"`, cell.Rowspan)
			}

			sb.WriteString(fmt.Sprintf("<td%s>%s</td>", attrs, cellText))
		}
		sb.WriteString("</tr>\n")
	}

	sb.WriteString("</table>")
	return sb.String()
}

// StyleCSS returns a CSS-like description for a style definition.
func (s *StyleDefinition) StyleCSS() string {
	var parts []string

	if s.RPr != nil {
		rp := s.RPr
		if rp.Bold {
			parts = append(parts, "font-weight:bold")
		}
		if rp.Italic {
			parts = append(parts, "font-style:italic")
		}
		if rp.FontSize > 0 {
			parts = append(parts, fmt.Sprintf("font-size:%.1fpt", rp.FontSize))
		}
		if rp.FontName != "" {
			parts = append(parts, fmt.Sprintf("font-family:%s", rp.FontName))
		}
		if rp.Color != "" {
			parts = append(parts, fmt.Sprintf("color:%s", rp.Color))
		}
	}

	if s.PPr != nil {
		pp := s.PPr
		if pp.Alignment != "" {
			parts = append(parts, fmt.Sprintf("text-align:%s", pp.Alignment))
		}
		if pp.SpaceBefore > 0 {
			parts = append(parts, fmt.Sprintf("margin-top:%.1fpt", pp.SpaceBefore))
		}
		if pp.SpaceAfter > 0 {
			parts = append(parts, fmt.Sprintf("margin-bottom:%.1fpt", pp.SpaceAfter))
		}
	}

	return strings.Join(parts, "; ")
}

// NumParagraphs returns the count of paragraph elements in the body.
func (doc *Document) NumParagraphs() int {
	count := 0
	for _, elem := range doc.Body {
		if elem.Kind == ElemKindParagraph {
			count++
		}
	}
	return count
}

// GetParagraphByIndex returns the paragraph at the given 0-based index across all body elements.
func (doc *Document) GetParagraphByIndex(idx int) *Paragraph {
	current := 0
	for _, elem := range doc.Body {
		if elem.Kind == ElemKindParagraph && elem.Para != nil {
			if current == idx {
				return elem.Para
			}
			current++
		}
	}
	return nil
}

// StyleUsage counts how many paragraphs use each style.
func (doc *Document) StyleUsage() map[string]int {
	usage := make(map[string]int)
	for _, elem := range doc.Body {
		if elem.Kind == ElemKindParagraph && elem.Para != nil {
			name := "Normal"
			if elem.Para.StyleName != "" {
				name = elem.Para.StyleName
			}
			usage[name]++
		}
	}
	return usage
}

// Overview returns a human-readable summary of the document structure.
func (doc *Document) Overview() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("段落数: %d\n", doc.NumParagraphs()))

	if len(doc.Body) == 0 {
		return sb.String()
	}

	tableCount := 0
	for _, elem := range doc.Body {
		if elem.Kind == ElemKindTable {
			tableCount++
		}
	}
	if tableCount > 0 {
		sb.WriteString(fmt.Sprintf("表格数: %d\n", tableCount))
	}

	usage := doc.StyleUsage()
	if len(usage) > 0 {
		sb.WriteString("\n样式使用情况:\n")
		for name, count := range usage {
			sb.WriteString(fmt.Sprintf("  %s: %d 个段落\n", name, count))
		}
	}

	return sb.String()
}
