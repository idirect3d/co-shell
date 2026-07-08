// Author: L.Shuang
// Created: 2026-07-08
// Last Modified: 2026-07-08
//
// MIT License
// Copyright (c) 2026 L.Shuang
package docx

import (
	"archive/zip"
	"fmt"
	"os"
	"strings"
)

// Save writes the in-memory document back to a DOCX file.
func (doc *Document) Save() error {
	if doc.Path == "" {
		return fmt.Errorf("document path is not set")
	}
	return doc.SaveAs(doc.Path)
}

// SaveAs writes the document to the specified path.
func (doc *Document) SaveAs(path string) error {
	// Build the new document.xml from body
	docXML := doc.buildDocumentXML()
	doc.Files["word/document.xml"] = []byte(docXML)

	// Write the ZIP
	return doc.writeZip(path)
}

// buildDocumentXML constructs word/document.xml from the body elements.
func (doc *Document) buildDocumentXML() string {
	var sb strings.Builder

	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`)
	sb.WriteString(` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`)
	sb.WriteString(`>`)
	sb.WriteString(`<w:body>`)

	for _, elem := range doc.Body {
		if elem.Kind == ElemKindParagraph {
			sb.WriteString(doc.buildParagraphXML(elem.Para))
		} else if elem.Kind == ElemKindTable {
			sb.WriteString(doc.buildTableXML(elem.Table))
		}
	}

	sb.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr>`)
	sb.WriteString(`</w:body>`)
	sb.WriteString(`</w:document>`)

	return sb.String()
}

// buildParagraphXML constructs XML for a single paragraph.
func (doc *Document) buildParagraphXML(p *Paragraph) string {
	var sb strings.Builder

	sb.WriteString(`<w:p>`)

	// Paragraph properties
	if p.StyleID != "" || p.PPr != nil {
		sb.WriteString(`<w:pPr>`)
		if p.StyleID != "" {
			sb.WriteString(fmt.Sprintf(`<w:pStyle w:val="%s"/>`, xmlEscape(p.StyleID)))
		}
		if p.PPr != nil {
			pp := doc.buildParaPropsXML(p.PPr)
			sb.WriteString(pp)
		}
		sb.WriteString(`</w:pPr>`)
	}

	// Runs
	for _, run := range p.Runs {
		sb.WriteString(doc.buildRunXML(run))
	}

	sb.WriteString(`</w:p>`)

	return sb.String()
}

// buildRunXML constructs XML for a single run.
func (doc *Document) buildRunXML(r *Run) string {
	var sb strings.Builder

	if r.Text == "" {
		return ""
	}

	sb.WriteString(`<w:r>`)

	// Run properties
	if r.RPr != nil && (r.RPr.Bold || r.RPr.Italic || r.RPr.Underline ||
		r.RPr.FontSize > 0 || r.RPr.FontName != "" || r.RPr.Color != "") {
		sb.WriteString(`<w:rPr>`)
		if r.RPr.Bold {
			sb.WriteString(`<w:b/>`)
		}
		if r.RPr.Italic {
			sb.WriteString(`<w:i/>`)
		}
		if r.RPr.Underline {
			sb.WriteString(`<w:u w:val="single"/>`)
		}
		if r.RPr.FontSize > 0 {
			halfPt := int(r.RPr.FontSize * 2)
			sb.WriteString(fmt.Sprintf(`<w:sz w:val="%d"/>`, halfPt))
			sb.WriteString(fmt.Sprintf(`<w:szCs w:val="%d"/>`, halfPt))
		}
		if r.RPr.FontName != "" {
			sb.WriteString(fmt.Sprintf(`<w:rFonts w:ascii="%s" w:hAnsi="%s"/>`, xmlEscape(r.RPr.FontName), xmlEscape(r.RPr.FontName)))
		}
		if r.RPr.Color != "" {
			colorVal := strings.TrimPrefix(r.RPr.Color, "#")
			sb.WriteString(fmt.Sprintf(`<w:color w:val="%s"/>`, xmlEscape(colorVal)))
		}
		sb.WriteString(`</w:rPr>`)
	}

	// Text
	text := xmlEscape(r.Text)
	sb.WriteString(fmt.Sprintf(`<w:t xml:space="preserve">%s</w:t>`, text))

	sb.WriteString(`</w:r>`)

	return sb.String()
}

// buildParaPropsXML constructs paragraph properties XML.
func (doc *Document) buildParaPropsXML(pp *ParaProps) string {
	var sb strings.Builder

	if pp.Alignment != "" {
		sb.WriteString(fmt.Sprintf(`<w:jc w:val="%s"/>`, xmlEscape(pp.Alignment)))
	}

	if pp.SpaceBefore > 0 || pp.SpaceAfter > 0 || pp.LineSpacing > 0 {
		sb.WriteString(`<w:spacing`)
		if pp.SpaceBefore > 0 {
			twips := int(pp.SpaceBefore * 20)
			sb.WriteString(fmt.Sprintf(` w:before="%d"`, twips))
		}
		if pp.SpaceAfter > 0 {
			twips := int(pp.SpaceAfter * 20)
			sb.WriteString(fmt.Sprintf(` w:after="%d"`, twips))
		}
		if pp.LineSpacing > 0 {
			line240 := int(pp.LineSpacing * 240)
			sb.WriteString(fmt.Sprintf(` w:line="%d" w:lineRule="auto"`, line240))
		}
		sb.WriteString(`/>`)
	}

	if pp.LeftIndent > 0 {
		twips := int(pp.LeftIndent * 567)
		sb.WriteString(fmt.Sprintf(`<w:ind w:left="%d"/>`, twips))
	}

	return sb.String()
}

// buildTableXML constructs XML for a table.
func (doc *Document) buildTableXML(t *Table) string {
	var sb strings.Builder

	sb.WriteString(`<w:tbl>`)

	// Table grid definition
	sb.WriteString(`<w:tblGrid>`)
	if len(t.Rows) > 0 && len(t.Rows[0].Cells) > 0 {
		for i := 0; i < len(t.Rows[0].Cells); i++ {
			sb.WriteString(`<w:gridCol w:w="2000"/>`)
		}
	}
	sb.WriteString(`</w:tblGrid>`)

	// Rows
	for _, row := range t.Rows {
		sb.WriteString(`<w:tr>`)
		for _, cell := range row.Cells {
			doc.buildTableCellXML(&sb, cell)
		}
		sb.WriteString(`</w:tr>`)
	}

	sb.WriteString(`</w:tbl>`)

	return sb.String()
}

// buildTableCellXML constructs XML for a table cell.
func (doc *Document) buildTableCellXML(sb *strings.Builder, cell *TableCell) {
	sb.WriteString(`<w:tc>`)

	// Cell properties
	if cell.Colspan > 1 || cell.Rowspan > 1 {
		sb.WriteString(`<w:tcPr>`)
		if cell.Colspan > 1 {
			sb.WriteString(fmt.Sprintf(`<w:gridSpan w:val="%d"/>`, cell.Colspan))
		}
		if cell.Rowspan > 1 {
			sb.WriteString(`<w:vMerge w:val="restart"/>`)
		}
		sb.WriteString(`</w:tcPr>`)
	}

	// Cell content as a simple paragraph
	text := xmlEscape(cell.Text)
	if text == "" {
		text = " "
	}
	sb.WriteString(fmt.Sprintf(`<w:p><w:r><w:t>%s</w:t></w:r></w:p>`, text))

	sb.WriteString(`</w:tc>`)
}

// writeZip writes the files map to a ZIP file.
func (doc *Document) writeZip(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create output file: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)

	// Write all files in a deterministic order
	order := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"word/_rels/document.xml.rels",
		"word/document.xml",
		"word/styles.xml",
	}

	// Write ordered files first
	written := make(map[string]bool)
	for _, name := range order {
		if data, ok := doc.Files[name]; ok {
			if err := addZipFile(zw, name, data); err != nil {
				return err
			}
			written[name] = true
		}
	}

	// Write remaining files
	for name, data := range doc.Files {
		if !written[name] {
			if err := addZipFile(zw, name, data); err != nil {
				return err
			}
		}
	}

	return zw.Close()
}

func addZipFile(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("cannot create zip entry %s: %w", name, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("cannot write zip entry %s: %w", name, err)
	}
	return nil
}

// CreateEmpty creates a new empty DOCX document in memory.
func CreateEmpty(path string) *Document {
	doc := &Document{
		Path:   path,
		Body:   nil,
		Styles: make(map[string]*StyleDefinition),
		Files:  make(map[string][]byte),
	}

	// Required OOXML boilerplate files
	doc.Files["[Content_Types].xml"] = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`)

	doc.Files["_rels/.rels"] = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`)

	doc.Files["word/_rels/document.xml.rels"] = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`)

	// Minimal styles.xml
	doc.Files["word/styles.xml"] = []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:styleId="Normal">
    <w:name w:val="Normal"/>
    <w:pPr>
      <w:spacing w:after="160" w:line="360" w:lineRule="auto"/>
    </w:pPr>
    <w:rPr>
      <w:sz w:val="24"/>
      <w:szCs w:val="24"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:next w:val="Normal"/>
    <w:pPr>
      <w:spacing w:before="360" w:after="120"/>
    </w:pPr>
    <w:rPr>
      <w:b/>
      <w:sz w:val="36"/>
      <w:szCs w:val="36"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/>
    <w:next w:val="Normal"/>
    <w:pPr>
      <w:spacing w:before="240" w:after="80"/>
    </w:pPr>
    <w:rPr>
      <w:b/>
      <w:sz w:val="30"/>
      <w:szCs w:val="30"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading3">
    <w:name w:val="heading 3"/>
    <w:next w:val="Normal"/>
    <w:pPr>
      <w:spacing w:before="200" w:after="60"/>
    </w:pPr>
    <w:rPr>
      <w:b/>
      <w:sz w:val="26"/>
      <w:szCs w:val="26"/>
    </w:rPr>
  </w:style>
</w:styles>`)

	// Default styles
	doc.Styles["Normal"] = &StyleDefinition{ID: "Normal", Name: "Normal", Type: "paragraph"}
	doc.Styles["Heading1"] = &StyleDefinition{ID: "Heading1", Name: "heading 1", Type: "paragraph"}
	doc.Styles["Heading2"] = &StyleDefinition{ID: "Heading2", Name: "heading 2", Type: "paragraph"}
	doc.Styles["Heading3"] = &StyleDefinition{ID: "Heading3", Name: "heading 3", Type: "paragraph"}

	return doc
}

// InsertParagraph creates a new paragraph and inserts it after the specified body element index.
// Returns the new paragraph.
func (doc *Document) InsertParagraph(afterIndex int, styleID, text string) *Paragraph {
	p := &Paragraph{
		Index:   afterIndex + 1,
		StyleID: styleID,
		Runs:    []*Run{{Text: text}},
	}

	elem := &DocElement{Kind: ElemKindParagraph, Para: p}

	if afterIndex < 0 {
		// Prepend
		doc.Body = append([]*DocElement{elem}, doc.Body...)
	} else if afterIndex >= len(doc.Body)-1 {
		// Append
		doc.Body = append(doc.Body, elem)
	} else {
		// Insert in the middle
		doc.Body = append(doc.Body, nil)
		copy(doc.Body[afterIndex+2:], doc.Body[afterIndex+1:])
		doc.Body[afterIndex+1] = elem
	}

	// Reindex paragraphs
	doc.reindexParas()

	return p
}

// RemoveParagraphRange removes paragraphs from start to end (inclusive, 0-based).
func (doc *Document) RemoveParagraphRange(startIdx, endIdx int) int {
	removed := 0
	// Find DocElements that are paragraphs in the range
	var keep []*DocElement
	for _, elem := range doc.Body {
		if elem.Kind == ElemKindParagraph && elem.Para != nil {
			if elem.Para.Index >= startIdx && elem.Para.Index <= endIdx {
				removed++
				continue
			}
		}
		keep = append(keep, elem)
	}
	doc.Body = keep
	doc.reindexParas()
	return removed
}

// reindexParas updates paragraph indices after modifications.
func (doc *Document) reindexParas() {
	idx := 0
	for _, elem := range doc.Body {
		if elem.Kind == ElemKindParagraph && elem.Para != nil {
			elem.Para.Index = idx
			idx++
		}
	}
}

// xmlEscape escapes special XML characters using numeric entities.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&#38;")
	s = strings.ReplaceAll(s, "<", "&#60;")
	s = strings.ReplaceAll(s, ">", "&#62;")
	s = strings.ReplaceAll(s, "\"", "&#34;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// AddParagraph adds a paragraph to the end of the document body.
func (doc *Document) AddParagraph(styleID, text string) *Paragraph {
	p := &Paragraph{
		StyleID: styleID,
		Runs:    []*Run{{Text: text}},
	}

	doc.Body = append(doc.Body, &DocElement{Kind: ElemKindParagraph, Para: p})
	doc.reindexParas()
	return p
}
