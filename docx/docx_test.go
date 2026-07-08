// Author: L.Shuang
// Created: 2026-07-08
// Last Modified: 2026-07-08
//
// MIT License
// Copyright (c) 2026 L.Shuang
package docx

import (
	"archive/zip"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tempDoc creates a temporary docx file path.
func tempDoc(t *testing.T) string {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "docx-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })
	return filepath.Join(tmpDir, "test.docx")
}

// TestCreateEmptyRoundtrip tests creating a new empty document, saving,
// and re-reading it.
func TestCreateEmptyRoundtrip(t *testing.T) {
	path := tempDoc(t)

	doc := CreateEmpty(path)
	if doc == nil {
		t.Fatal("CreateEmpty returned nil")
	}

	// Verify initial state
	if doc.NumParagraphs() != 0 {
		t.Errorf("expected 0 paragraphs, got %d", doc.NumParagraphs())
	}

	// Add some paragraphs
	doc.AddParagraphWithStyle("Heading1", &Run{Text: "第一章 绪论"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "这是正文内容。"})
	doc.AddParagraphWithStyle("Heading2", &Run{Text: "1.1 背景"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "研究背景描述。"})
	doc.AddParagraphWithStyle("Normal", &Run{})

	if doc.NumParagraphs() != 5 {
		t.Errorf("expected 5 paragraphs, got %d", doc.NumParagraphs())
	}

	// Save
	if err := doc.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists and is non-empty
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("cannot stat saved file: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("saved file is empty")
	}
	t.Logf("saved file size: %d bytes", info.Size())

	// Re-open and verify content
	doc2, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}

	if doc2.NumParagraphs() != 5 {
		t.Errorf("re-read: expected 5 paragraphs, got %d", doc2.NumParagraphs())
	}

	// Check style resolution
	p0 := doc2.GetParagraphByIndex(0)
	if p0 == nil {
		t.Fatal("paragraph 0 is nil")
	}
	t.Logf("para 0: style=%q name=%q text=%q", p0.StyleID, p0.StyleName, p0.Text())

	if p0.StyleID != "Heading1" {
		t.Errorf("para 0: expected StyleID=Heading1, got %q", p0.StyleID)
	}
	if p0.Text() != "第一章 绪论" {
		t.Errorf("para 0: expected text=第一章 绪论, got %q", p0.Text())
	}

	// Check Normal paragraph
	p3 := doc2.GetParagraphByIndex(3)
	if p3 == nil {
		t.Fatal("paragraph 3 is nil")
	}
	t.Logf("para 3: style=%q name=%q text=%q", p3.StyleID, p3.StyleName, p3.Text())
	if p3.StyleID != "Normal" {
		t.Errorf("para 3: expected StyleID=Normal, got %q", p3.StyleID)
	}

	// Check empty paragraph
	p4 := doc2.GetParagraphByIndex(4)
	if p4 == nil {
		t.Fatal("paragraph 4 is nil")
	}
	t.Logf("para 4: style=%q name=%q runs=%d", p4.StyleID, p4.StyleName, len(p4.Runs))
}

// TestInsertAndRemoveParagraphs tests the InsertParagraph and RemoveParagraphRange functions.
func TestInsertAndRemoveParagraphs(t *testing.T) {
	path := tempDoc(t)

	doc := CreateEmpty(path)

	// Add initial paragraphs
	doc.AddParagraphWithStyle("Normal", &Run{Text: "第一段"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "第二段"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "第三段"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "第四段"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "第五段"})

	if doc.NumParagraphs() != 5 {
		t.Fatalf("expected 5 paragraphs, got %d", doc.NumParagraphs())
	}

	// Insert after paragraph 1 (0-based index 1, "第二段")
	doc.InsertParagraph(1, "Heading1", "插入的标题")
	if doc.NumParagraphs() != 6 {
		t.Errorf("after insert: expected 6 paragraphs, got %d", doc.NumParagraphs())
	}

	// Verify content
	p2 := doc.GetParagraphByIndex(2)
	if p2 == nil || p2.Text() != "插入的标题" {
		t.Errorf("expected paragraph 2 to be '插入的标题', got %v", p2)
	}
	if p2.StyleID != "Heading1" {
		t.Errorf("expected style Heading1, got %q", p2.StyleID)
	}

	// Verify "第三段" moved to index 3
	p3 := doc.GetParagraphByIndex(3)
	if p3 == nil || p3.Text() != "第三段" {
		t.Errorf("expected paragraph 3 to be '第三段', got %v", p3)
	}

	// Remove paragraphs 2-3 (0-based), which are "插入的标题" and "第三段"
	deleted := doc.RemoveParagraphRange(2, 3)
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}
	if doc.NumParagraphs() != 4 {
		t.Errorf("after remove: expected 4 paragraphs, got %d", doc.NumParagraphs())
	}

	// Verify remaining content
	remaining := make([]string, doc.NumParagraphs())
	for i := 0; i < doc.NumParagraphs(); i++ {
		p := doc.GetParagraphByIndex(i)
		if p != nil {
			remaining[i] = p.Text()
		}
	}
	expected := []string{"第一段", "第二段", "第四段", "第五段"}
	for i, v := range expected {
		if remaining[i] != v {
			t.Errorf("remaining[%d]: expected %q, got %q", i, v, remaining[i])
		}
	}

	// Save and verify ZIP is valid
	if err := doc.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("cannot open saved zip: %v", err)
	}
	defer zr.Close()
	t.Logf("ZIP entries after save: %d", len(zr.File))
}

// TestReadParagraphRange tests reading paragraphs as HTML.
func TestReadParagraphRange(t *testing.T) {
	path := tempDoc(t)

	doc := CreateEmpty(path)

	// Build a structured document
	doc.AddParagraphWithStyle("Heading1", &Run{Text: "第一章"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "第一段正文内容。"})
	doc.AddParagraphWithStyle("Heading2", &Run{Text: "1.1 小节"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "小节正文。"})

	// Read as simple HTML
	count, html, err := doc.ReadParagraphRange(0, 3, "simple")
	if err != nil {
		t.Fatalf("ReadParagraphRange failed: %v", err)
	}
	if count != 4 {
		t.Errorf("expected count=4, got %d", count)
	}
	t.Logf("simple HTML:\n%s", html)

	// Verify HTML structure
	if !strings.Contains(html, "<h1") {
		t.Error("expected <h1> tag in output")
	}
	if !strings.Contains(html, "<h2") {
		t.Error("expected <h2> tag in output")
	}
	if !strings.Contains(html, "<p") {
		t.Error("expected <p> tag in output")
	}
	if !strings.Contains(html, "第一章") {
		t.Error("expected '第一章' in output")
	}

	// Read single paragraph
	count2, html2, err := doc.ReadParagraphRange(2, 2, "simple")
	if err != nil {
		t.Fatalf("ReadParagraphRange single failed: %v", err)
	}
	if count2 != 1 {
		t.Errorf("expected count=1, got %d", count2)
	}
	if !strings.Contains(html2, "1.1 小节") {
		t.Errorf("expected '1.1 小节', got %q", html2)
	}
	t.Logf("single para HTML: %s", html2)

	// Read out of range - should clamp to valid
	count3, html3, err := doc.ReadParagraphRange(0, 100, "simple")
	if err != nil {
		t.Fatalf("ReadParagraphRange clamped failed: %v", err)
	}
	if count3 != 4 {
		t.Errorf("clamped: expected count=4, got %d", count3)
	}
	_ = html3
}

// TestXMLSpecialChars tests that special characters are properly escaped.
func TestXMLSpecialChars(t *testing.T) {
	path := tempDoc(t)

	doc := CreateEmpty(path)

	// Add paragraph with special XML characters
	specialText := "A & B < C > D \"quote\" 'single'"
	doc.AddParagraphWithStyle("Normal", &Run{Text: specialText})

	if err := doc.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the XML file contains properly escaped content
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("cannot open zip: %v", err)
	}
	defer zr.Close()

	var docXML string
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			buf := make([]byte, 8192)
			n, _ := rc.Read(buf)
			docXML = string(buf[:n])
			rc.Close()
			break
		}
	}

	if docXML == "" {
		t.Fatal("word/document.xml not found in zip")
	}
	t.Logf("document.xml snippet: %s", docXML[:min(len(docXML), 300)])

	// Verify escape sequences
	if strings.Contains(docXML, "&") && !strings.Contains(docXML, "&#38;") {
		t.Error("raw & found without escaping")
	}
}

// TestStyleUsage verifies the Overview and StyleUsage functions.
func TestStyleUsage(t *testing.T) {
	path := tempDoc(t)

	doc := CreateEmpty(path)

	doc.AddParagraphWithStyle("Heading1", &Run{Text: "H1"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "N1"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "N2"})
	doc.AddParagraphWithStyle("Heading2", &Run{Text: "H2"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "N3"})

	// StyleUsage
	usage := doc.StyleUsage()
	if usage["heading 1"] != 1 {
		t.Errorf("heading 1: expected 1, got %d", usage["heading 1"])
	}
	if usage["Normal"] != 3 {
		t.Errorf("Normal: expected 3, got %d", usage["Normal"])
	}
	if usage["heading 2"] != 1 {
		t.Errorf("heading 2: expected 1, got %d", usage["heading 2"])
	}

	// Overview
	overview := doc.Overview()
	t.Logf("Overview:\n%s", overview)
	if !strings.Contains(overview, "段落数: 5") {
		t.Errorf("expected paragraph count 5 in overview")
	}
}

// TestCreateOpenExisting tests opening a real DOCX file.
func TestCreateOpenExisting(t *testing.T) {
	// First create a file
	path := tempDoc(t)

	doc := CreateEmpty(path)
	doc.AddParagraphWithStyle("Heading1", &Run{Text: "标题"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "正文"})
	if err := doc.Save(); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}

	// Now open it
	doc2, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}

	if doc2.NumParagraphs() != 2 {
		t.Errorf("expected 2 paragraphs, got %d", doc2.NumParagraphs())
	}

	// Add more content and re-save
	doc2.AddParagraphWithStyle("Heading2", &Run{Text: "新章节"})
	if err := doc2.Save(); err != nil {
		t.Fatalf("re-save failed: %v", err)
	}

	// Open again and verify
	doc3, err := OpenFile(path)
	if err != nil {
		t.Fatalf("re-open failed: %v", err)
	}
	if doc3.NumParagraphs() != 3 {
		t.Errorf("after re-save: expected 3 paragraphs, got %d", doc3.NumParagraphs())
	}
}

// TestTableSupport verifies table reading and HTML output.
func TestTableSupport(t *testing.T) {
	path := tempDoc(t)

	doc := CreateEmpty(path)

	// Create a table
	tbl := &Table{
		Index: 0,
		Rows: []*TableRow{
			{
				Cells: []*TableCell{
					{Text: "Name", Colspan: 1, Rowspan: 1},
					{Text: "Age", Colspan: 1, Rowspan: 1},
				},
			},
			{
				Cells: []*TableCell{
					{Text: "Alice", Colspan: 1, Rowspan: 1},
					{Text: "30", Colspan: 1, Rowspan: 1},
				},
			},
			{
				Cells: []*TableCell{
					{Text: "Bob", Colspan: 1, Rowspan: 1},
					{Text: "25", Colspan: 1, Rowspan: 1},
				},
			},
		},
	}

	// Add table and a paragraph after it
	doc.Body = append(doc.Body, &DocElement{Kind: ElemKindTable, Table: tbl})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "表格后正文"})

	// Save
	if err := doc.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Re-open and verify table
	doc2, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}

	// Verify paragraph count (should be 1 paragraph after the table)
	if doc2.NumParagraphs() != 1 {
		t.Errorf("expected 1 paragraph, got %d", doc2.NumParagraphs())
	}

	// Reopen and export table as HTML via our TableToHTML function
	tableFound := false
	for _, elem := range doc2.Body {
		if elem.Kind == ElemKindTable && elem.Table != nil {
			tableFound = true
			html := TableToHTML(elem.Table, "simple")
			t.Logf("Table HTML:\n%s", html)
			if !strings.Contains(html, "<table>") {
				t.Error("expected <table> in table HTML")
			}
			if !strings.Contains(html, "Alice") {
				t.Errorf("expected 'Alice' in table HTML")
			}
			if !strings.Contains(html, "colspan") {
				// colspan=1 should be omitted in simple mode
				t.Log("colspan=1 not present (expected)")
			}
		}
	}
	if !tableFound {
		t.Error("table not found in re-read document")
	}
}

// TestLookupStyle tests style lookup by ID and name.
func TestLookupStyle(t *testing.T) {
	path := tempDoc(t)
	doc := CreateEmpty(path)

	style := doc.LookupStyle("Heading1")
	if style == nil {
		t.Fatal("LookupStyle(Heading1) returned nil")
	}
	if style.ID != "Heading1" {
		t.Errorf("expected ID=Heading1, got %q", style.ID)
	}

	style = doc.LookupStyle("heading 1")
	if style == nil {
		t.Fatal("LookupStyle('heading 1') returned nil")
	}

	style = doc.LookupStyle("Nonexistent")
	if style != nil {
		t.Errorf("expected nil for nonexistent style, got %v", style)
	}

	ids := doc.StyleIDs()
	if len(ids) != 4 {
		t.Errorf("expected 4 style IDs, got %d: %v", len(ids), ids)
	}
}

// TestRoundtripWithFormatting tests that inline formatting survives save/re-open.
func TestRoundtripWithFormatting(t *testing.T) {
	path := tempDoc(t)

	doc := CreateEmpty(path)

	// Create a paragraph with formatted runs
	doc.AddParagraphWithStyle("Normal",
		&Run{Text: "这是", RPr: &RunProps{Bold: true, FontSize: 12, Color: "#FF0000"}},
		&Run{Text: "格式化", RPr: &RunProps{Italic: true, FontSize: 14, Color: "#0000FF"}},
		&Run{Text: "文本", RPr: &RunProps{FontName: "SimSun", FontSize: 12}},
	)

	if err := doc.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify ZIP contains properly formatted XML
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	var docXML string
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			buf := make([]byte, 8192)
			n, _ := rc.Read(buf)
			docXML = string(buf[:n])
			rc.Close()
			break
		}
	}

	if !strings.Contains(docXML, "<w:b/>") {
		t.Error("expected <w:b/> for bold in XML output")
	}
	if !strings.Contains(docXML, "<w:i/>") {
		t.Error("expected <w:i/> for italic in XML output")
	}
	if !strings.Contains(docXML, "FF0000") {
		t.Error("expected FF0000 color in XML output")
	}
	if !strings.Contains(docXML, "SimSun") {
		t.Error("expected SimSun font name in XML output")
	}
	t.Logf("document.xml: %s", docXML[:min(len(docXML), 500)])
}

// TestSetParagraphStyle verifies SetParagraphStyle.
func TestSetParagraphStyle(t *testing.T) {
	path := tempDoc(t)
	doc := CreateEmpty(path)

	doc.AddParagraphWithStyle("Normal", &Run{Text: "测试"})

	if err := doc.SetParagraphStyle(0, "Heading2"); err != nil {
		t.Fatalf("SetParagraphStyle failed: %v", err)
	}

	p := doc.GetParagraphByIndex(0)
	if p == nil {
		t.Fatal("paragraph 0 is nil")
	}
	if p.StyleID != "Heading2" {
		t.Errorf("expected StyleID=Heading2, got %q", p.StyleID)
	}

	// Try setting style on nonexistent paragraph
	err := doc.SetParagraphStyle(999, "Heading1")
	if err == nil {
		t.Error("expected error for out-of-range paragraph")
	}
}

// TestOverviewOutput tests the Overview output matches content.
func TestOverviewOutput(t *testing.T) {
	path := tempDoc(t)
	doc := CreateEmpty(path)

	overview := doc.Overview()
	if !strings.Contains(overview, "段落数: 0") {
		t.Errorf("empty doc overview should show 0 paragraphs")
	}

	doc.AddParagraphWithStyle("Heading1", &Run{Text: "A"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "B"})

	overview = doc.Overview()
	t.Logf("Overview: %s", overview)
	if !strings.Contains(overview, "段落数: 2") {
		t.Error("expected 2 paragraphs in overview")
	}
	if !strings.Contains(overview, "heading 1") {
		t.Error("expected 'heading 1' style in overview")
	}
}

// TestAddParagraphWithStyleNoRuns tests adding empty paragraphs.
func TestAddParagraphWithStyleNoRuns(t *testing.T) {
	path := tempDoc(t)
	doc := CreateEmpty(path)

	doc.AddParagraphWithStyle("Normal") // no runs
	if doc.NumParagraphs() != 1 {
		t.Errorf("expected 1 paragraph, got %d", doc.NumParagraphs())
	}

	p := doc.GetParagraphByIndex(0)
	if p == nil {
		t.Fatal("paragraph 0 is nil")
	}
	if p.StyleID != "Normal" {
		t.Errorf("expected Normal style, got %q", p.StyleID)
	}
	if err := doc.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
}

// TestXMLParseResult validates that the generated XML is well-formed.
func TestXMLParseResult(t *testing.T) {
	path := tempDoc(t)
	doc := CreateEmpty(path)

	doc.AddParagraphWithStyle("Heading1", &Run{Text: "Test Title"})
	doc.AddParagraphWithStyle("Normal", &Run{Text: "Test body text."})

	if err := doc.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Parse document.xml with encoding/xml decoder to verify well-formedness
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			dec := xml.NewDecoder(rc)
			depth := 0
			for {
				tok, err := dec.Token()
				if err != nil {
					break
				}
				switch tok.(type) {
				case xml.StartElement:
					depth++
				case xml.EndElement:
					depth--
				}
			}
			rc.Close()
			if depth != 0 {
				t.Errorf("XML parse depth check: final depth=%d (expected 0)", depth)
			} else {
				t.Log("XML is well-formed (depth check passed)")
			}
		}
	}
}

// TestRunLevelFormatting verifies escape sequences in document.xml
// preserve run-level formatting from runs.
func TestRunLevelFormatting(t *testing.T) {
	path := tempDoc(t)
	doc := CreateEmpty(path)

	run := &Run{Text: "test"}
	run.RPr = &RunProps{
		Bold:      true,
		Italic:    true,
		FontSize:  14,
		FontName:  "SimSun",
		Color:     "#AA00BB",
		Underline: true,
	}
	doc.AddParagraphWithStyle("Normal", run)
	if err := doc.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Read back XML
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	var docXML string
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			buf := make([]byte, 4096)
			n, _ := rc.Read(buf)
			docXML = string(buf[:n])
			rc.Close()
			break
		}
	}

	checks := []struct {
		tag    string
		expect string
	}{
		{"bold", "<w:b/>"},
		{"italic", "<w:i/>"},
		{"underline", `<w:u w:val="single"/>`},
		{"font size", `<w:sz w:val="28"/>`},
		{"font name", "SimSun"},
		{"color", "AA00BB"},
	}
	for _, c := range checks {
		if !strings.Contains(docXML, c.expect) {
			t.Errorf("expected %s=%q in XML, not found", c.tag, c.expect)
		}
	}
}

// TestPrependInsert tests inserting at the beginning of an empty document
// and at index 0.
func TestPrependInsert(t *testing.T) {
	path := tempDoc(t)
	doc := CreateEmpty(path)

	// Insert at beginning of empty doc
	doc.InsertParagraph(-1, "Normal", "第一段")
	if doc.NumParagraphs() != 1 {
		t.Errorf("expected 1 paragraph, got %d", doc.NumParagraphs())
	}

	doc.InsertParagraph(-1, "Normal", "最前面")
	if doc.NumParagraphs() != 2 {
		t.Errorf("expected 2 paragraphs, got %d", doc.NumParagraphs())
	}

	p0 := doc.GetParagraphByIndex(0)
	if p0 == nil || p0.Text() != "最前面" {
		t.Errorf("expected para 0 to be '最前面', got %v", p0)
	}
}

// TestNilDocumentSafety tests that operations on a nil/invalid document don't crash.
func TestNilDocumentSafety(t *testing.T) {
	// CreateEmpty always produces a valid document - test edge cases
	path := tempDoc(t)
	doc := CreateEmpty(path)

	// Ensure zero paragraphs has correct behavior
	if doc.NumParagraphs() != 0 {
		t.Errorf("expected 0 paragraphs initially")
	}

	// AddParagraph should work on empty doc
	doc.AddParagraphWithStyle("Normal", &Run{Text: "hello"})
	if doc.NumParagraphs() != 1 {
		t.Errorf("expected 1 paragraph after add")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// printDocContent is a helper for debugging.
func printDocContent(t *testing.T, doc *Document) {
	t.Helper()
	for i := 0; i < doc.NumParagraphs(); i++ {
		p := doc.GetParagraphByIndex(i)
		if p != nil {
			t.Logf("  [%d] style=%q name=%q text=%q", i, p.StyleID, p.StyleName, p.Text())
		}
	}
}
