// Author: L.Shuang
// Created: 2026-07-08
// Last Modified: 2026-07-08
//
// MIT License
// Copyright (c) 2026 L.Shuang
package docx

import (
	"fmt"
	"strings"
)

// Styles returns the list of available paragraph style IDs.
func (doc *Document) StyleIDs() []string {
	var ids []string
	for id := range doc.Styles {
		ids = append(ids, id)
	}
	return ids
}

// LookupStyle finds a style by name or ID.
func (doc *Document) LookupStyle(nameOrID string) *StyleDefinition {
	// Try ID first
	if s, ok := doc.Styles[nameOrID]; ok {
		return s
	}
	// Try name
	for _, s := range doc.Styles {
		if s.Name == nameOrID {
			return s
		}
	}
	return nil
}

// ReadParagraphRange returns a range of paragraphs in the specified format.
// fromIdx and toIdx are 0-based paragraph indices.
// format: "simple" (HTML), "full" (HTML+CSS), "text" (plain), "md" (markdown)
// Returns count, content, error.
func (doc *Document) ReadParagraphRange(fromIdx, toIdx int, format string) (int, string, error) {
	if fromIdx < 0 {
		fromIdx = 0
	}
	if toIdx >= doc.NumParagraphs() {
		toIdx = doc.NumParagraphs() - 1
	}
	if toIdx < fromIdx {
		return 0, "", fmt.Errorf("toIdx (%d) must be >= fromIdx (%d)", toIdx, fromIdx)
	}

	count := toIdx - fromIdx + 1
	if count > 200 {
		return 0, "", fmt.Errorf("requested %d paragraphs, maximum is 200", count)
	}

	var parts []string
	for i := fromIdx; i <= toIdx; i++ {
		p := doc.GetParagraphByIndex(i)
		if p == nil {
			continue
		}
		var s string
		switch format {
		case "simple", "full":
			s = ParaToHTML(p, format)
		case "text":
			s = ParaToText(p)
		case "md":
			s = ParaToMarkdown(p)
		default:
			s = ParaToHTML(p, "simple")
		}
		if s != "" {
			parts = append(parts, s)
		}
	}

	return count, strings.Join(parts, "\n"), nil
}

// AddParagraphWithStyle adds a paragraph with the given style and text.
func (doc *Document) AddParagraphWithStyle(styleID string, runs ...*Run) *Paragraph {
	p := &Paragraph{
		StyleID: styleID,
		Runs:    runs,
	}

	// Resolve stylename
	if s, ok := doc.Styles[styleID]; ok {
		p.StyleName = s.Name
	}

	doc.Body = append(doc.Body, &DocElement{Kind: ElemKindParagraph, Para: p})
	doc.reindexParas()
	return p
}

// SetParagraphStyle sets the style of a paragraph by index.
func (doc *Document) SetParagraphStyle(idx int, styleID string) error {
	p := doc.GetParagraphByIndex(idx)
	if p == nil {
		return fmt.Errorf("paragraph %d not found", idx)
	}
	p.StyleID = styleID
	if s, ok := doc.Styles[styleID]; ok {
		p.StyleName = s.Name
	}
	return nil
}
