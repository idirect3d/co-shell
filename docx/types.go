// Package docx provides pure Go DOCX file parsing and writing.
// It implements the Office Open XML WordprocessingML format using only
// the Go standard library (archive/zip, encoding/xml).
//
// Author: L.Shuang
// Created: 2026-07-08
// Last Modified: 2026-07-08
//
// MIT License
// Copyright (c) 2026 L.Shuang
package docx

// DocElemKind indicates the kind of element in the document body.
type DocElemKind int

const (
	ElemKindParagraph DocElemKind = iota
	ElemKindTable
)

// DocElement is a sequential element in the document body (paragraph or table).
type DocElement struct {
	Kind  DocElemKind
	Para  *Paragraph
	Table *Table
}

// Paragraph represents a single paragraph (w:p) in the document.
type Paragraph struct {
	Index     int        // 0-based paragraph index in document
	StyleID   string     // style reference ID (e.g. "Heading1", "Normal")
	StyleName string     // human-readable style name (from styles.xml)
	Runs      []*Run     // text runs in order
	PPr       *ParaProps // paragraph-level properties
	RawXML    string     // original XML for preservation on save (if unmodified)
}

// Run represents a single run (w:r) containing formatted text.
type Run struct {
	Text   string    // the text content
	RPr    *RunProps // run-level formatting
	RawXML string    // original XML for preservation
}

// ParaProps holds paragraph-level formatting properties.
type ParaProps struct {
	Alignment       string  // left/center/right/justify
	SpaceBefore     float64 // in points
	SpaceAfter      float64 // in points
	LineSpacing     float64 // multiplier
	LeftIndent      float64 // in cm
	FirstLineIndent float64 // in points
}

// RunProps holds run-level character formatting properties.
type RunProps struct {
	Bold             bool
	Italic           bool
	Underline        bool
	FontSize         float64 // in points
	FontName         string
	FontNameEastAsia string
	Color            string // #RRGGBB
}

// Table represents a WORD table (w:tbl).
type Table struct {
	Index int
	Rows  []*TableRow
}

// TableRow represents a table row (w:tr).
type TableRow struct {
	Cells []*TableCell
}

// TableCell represents a table cell (w:tc).
type TableCell struct {
	Text    string
	Colspan int // w:gridSpan
	Rowspan int // w:vMerge (approximate)
	StyleID string
	RawXML  string
}

// StyleDefinition represents a named style from styles.xml.
type StyleDefinition struct {
	ID        string     // styleId attribute
	Name      string     // w:name val
	Type      string     // w:type (paragraph/character/table)
	RPr       *RunProps  // run-level defaults
	PPr       *ParaProps // paragraph-level defaults
	NextStyle string     // w:next style ID
	RawXML    string     // full raw XML for preservation
}

// HBorder defines a single border edge in table/paragraph.
type HBorder struct {
	Style string // single/thick/dashed/dotted
	Color string
	Size  int // in eighths of a point
}

// Document represents an in-memory DOCX document.
type Document struct {
	Path   string
	Body   []*DocElement               // sequential paragraphs and tables
	Styles map[string]*StyleDefinition // styleId -> style
	Files  map[string][]byte           // raw ZIP files (for preservation of unmodified parts)
}

// MergeCell represents a merged cell range.
type MergeCell struct {
	StartRow int
	StartCol int
	EndRow   int
	EndCol   int
}
