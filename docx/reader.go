// Author: L.Shuang
// Created: 2026-07-08
// Last Modified: 2026-07-08
//
// MIT License
// Copyright (c) 2026 L.Shuang
package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// namespace prefixes used in WordprocessingML
const (
	nsW   = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsR   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	nsWP  = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	nsMC  = "http://schemas.openxmlformats.org/markup-compatibility/2006"
	nsXML = "http://www.w3.org/XML/1998/namespace"
)

// attrVal extracts attribute value for a given namespace+local name.
func attrVal(attrs []xml.Attr, ns, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local && (a.Name.Space == ns || a.Name.Space == "") {
			return a.Value
		}
	}
	return ""
}

// parseTextTokens parses XML tokens from a reader and extracts <w:t> text content.
// Returns the text content and a closure that can re-serialize the tokens.
func parseTokensToXML(r *xml.Decoder, endTag xml.Name) ([]byte, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)

	depth := 0
	for {
		tok, err := r.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			enc.EncodeToken(t)
		case xml.EndElement:
			depth--
			if depth < 0 && t.Name == endTag {
				enc.Flush()
				return buf.Bytes(), nil
			}
			enc.EncodeToken(t)
		case xml.CharData:
			enc.EncodeToken(t)
		case xml.Comment:
			enc.EncodeToken(t)
		case xml.ProcInst:
			enc.EncodeToken(t)
		case xml.Directive:
			enc.EncodeToken(t)
		}
	}
	enc.Flush()
	return buf.Bytes(), nil
}

// collectTextTokens collects text from <w:t> elements in a token stream until the end tag.
func collectTextTokens(r *xml.Decoder, endTag xml.Name) (string, error) {
	var textParts []string
	depth := 0
	for {
		tok, err := r.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "t" && (t.Name.Space == nsW || t.Name.Space == "") {
				// Read CharData inside <w:t>
				inner, err := r.Token()
				if err == nil {
					if cd, ok := inner.(xml.CharData); ok {
						textParts = append(textParts, string(cd))
					}
				}
			}
		case xml.EndElement:
			depth--
			if depth < 0 && t.Name == endTag {
				return strings.Join(textParts, ""), nil
			}
		}
	}
	return strings.Join(textParts, ""), nil
}

// OpenFile opens a DOCX file and parses its content.
func OpenFile(path string) (*Document, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open docx zip: %w", err)
	}

	doc := &Document{
		Path:   path,
		Styles: make(map[string]*StyleDefinition),
		Body:   nil,
		Files:  make(map[string][]byte),
	}
	defer func() {
		if doc.Body == nil {
			zr.Close()
		}
	}()

	// Read all files into memory
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("cannot read %s in zip: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", f.Name, err)
		}
		doc.Files[f.Name] = data
	}

	// Parse styles.xml
	if data, ok := doc.Files["word/styles.xml"]; ok {
		if err := parseStyles(doc, data); err != nil {
			// Non-fatal: continue without styles
			_ = err
		}
	}

	// Parse document.xml
	data, ok := doc.Files["word/document.xml"]
	if !ok {
		return nil, fmt.Errorf("word/document.xml not found in docx")
	}

	if err := parseDocument(doc, data); err != nil {
		return nil, fmt.Errorf("cannot parse document.xml: %w", err)
	}

	zr.Close()
	return doc, nil
}

// parseStyles reads styles.xml and populates doc.Styles.
func parseStyles(doc *Document, data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))

	var currentStyle *StyleDefinition
	depth := 0
	inStyle := false
	collectRaw := false

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++

			if t.Name.Local == "style" && (t.Name.Space == nsW || t.Name.Space == "") {
				inStyle = true
				styleID := attrVal(t.Attr, nsW, "styleId")
				if styleID == "" {
					styleID = attrVal(t.Attr, "", "w:styleId")
				}
				currentStyle = &StyleDefinition{
					ID:   styleID,
					Type: attrVal(t.Attr, nsW, "type"),
				}
				collectRaw = true
			} else if inStyle && currentStyle != nil {
				if t.Name.Local == "name" && (t.Name.Space == nsW || t.Name.Space == "") {
					currentStyle.Name = attrVal(t.Attr, nsW, "val")
				} else if t.Name.Local == "next" && (t.Name.Space == nsW || t.Name.Space == "") {
					currentStyle.NextStyle = attrVal(t.Attr, nsW, "val")
				} else if t.Name.Local == "rPr" && (t.Name.Space == nsW || t.Name.Space == "") {
					currentStyle.RPr = parseRunProps(dec, t.Name)
				} else if t.Name.Local == "pPr" && (t.Name.Space == nsW || t.Name.Space == "") {
					currentStyle.PPr = parseParaProps(dec, t.Name)
				}
			}

		case xml.EndElement:
			depth--
			if inStyle && t.Name.Local == "style" && (t.Name.Space == nsW || t.Name.Space == "") {
				inStyle = false
				if currentStyle != nil {
					// Only store paragraph/character styles
					if currentStyle.Type == "paragraph" || currentStyle.Type == "" {
						doc.Styles[currentStyle.ID] = currentStyle
					}
					currentStyle = nil
				}
				collectRaw = false
			}
			_ = collectRaw

		case xml.CharData:
			_ = t // ignore cdata
		}
	}

	return nil
}

// parseDocument reads word/document.xml and populates doc.Body.
// In OOXML, body-level <w:p> and <w:tbl> elements are always flat siblings (never nested),
// so we can use simple string search for their closing tags.
func parseDocument(doc *Document, data []byte) error {
	content := string(data)

	// Find body content between <w:body> and </w:body>
	bodyStart := strings.Index(content, "<w:body")
	if bodyStart < 0 {
		bodyStart = strings.Index(content, "<w:body>")
	}
	if bodyStart < 0 {
		return fmt.Errorf("cannot find <w:body> in document.xml")
	}
	bodyStart = strings.IndexByte(content[bodyStart:], '>') + bodyStart + 1
	bodyEnd := strings.LastIndex(content, "</w:body>")
	if bodyEnd < 0 {
		return fmt.Errorf("cannot find </w:body> in document.xml")
	}

	paraIdx := 0
	tableIdx := 0
	i := bodyStart

	for i < bodyEnd {
		// Find next < character
		if i >= len(content) {
			break
		}
		ltIdx := strings.IndexByte(content[i:], '<')
		if ltIdx < 0 {
			break
		}
		ltIdx += i

		if ltIdx >= bodyEnd {
			break
		}

		// Determine element type
		// For <w:p> elements: find the first </w:p> after <w:p>
		if ltIdx+3 <= len(content) && strings.HasPrefix(content[ltIdx:], "<w:p") {
			start := ltIdx
			closeTag := "</w:p>"
			closeIdx := strings.Index(content[ltIdx:], closeTag)
			if closeIdx < 0 {
				i = ltIdx + 1
				continue
			}
			end := start + closeIdx + len(closeTag)
			if end > bodyEnd {
				end = bodyEnd
			}
			xmlBlock := content[start:end]

			para := parseParagraphFromXML(xmlBlock)
			para.Index = paraIdx
			paraIdx++
			if s, ok := doc.Styles[para.StyleID]; ok {
				para.StyleName = s.Name
			}
			doc.Body = append(doc.Body, &DocElement{
				Kind: ElemKindParagraph,
				Para: para,
			})
			i = end
		} else if ltIdx+5 <= len(content) && strings.HasPrefix(content[ltIdx:], "<w:tbl") {
			start := ltIdx
			closeTag := "</w:tbl>"
			closeIdx := strings.Index(content[ltIdx:], closeTag)
			if closeIdx < 0 {
				i = ltIdx + 1
				continue
			}
			end := start + closeIdx + len(closeTag)
			if end > bodyEnd {
				end = bodyEnd
			}
			xmlBlock := content[start:end]

			tbl, _ := parseTableFromXML(xmlBlock)
			tbl.Index = tableIdx
			tableIdx++
			doc.Body = append(doc.Body, &DocElement{
				Kind:  ElemKindTable,
				Table: tbl,
			})
			i = end
		} else {
			i = ltIdx + 1
		}
	}

	return nil
}

// parseParagraphFromXML parses a raw <w:p> XML block into a Paragraph.
func parseParagraphFromXML(xmlBlock string) *Paragraph {
	para := &Paragraph{}

	// Extract StyleID from <w:pStyle w:val="..."/>
	if idx := strings.Index(xmlBlock, "<w:pStyle"); idx >= 0 {
		rest := xmlBlock[idx:]
		valIdx := strings.Index(rest, `w:val="`)
		if valIdx >= 0 {
			valStart := valIdx + len(`w:val="`)
			valEnd := strings.IndexByte(rest[valStart:], '"')
			if valEnd >= 0 {
				styleID := rest[valStart : valStart+valEnd]
				if styleID != "" {
					para.StyleID = styleID
				}
			}
		}
	}

	// Extract runs: find all <w:t> text content
	runStart := 0
	for {
		tIdx := strings.Index(xmlBlock[runStart:], "<w:t")
		if tIdx < 0 {
			break
		}
		tagEnd := strings.IndexByte(xmlBlock[runStart+tIdx:], '>')
		if tagEnd < 0 {
			break
		}
		textStart := runStart + tIdx + tagEnd + 1
		closeIdx := strings.Index(xmlBlock[textStart:], "</w:t>")
		if closeIdx < 0 {
			break
		}
		text := xmlBlock[textStart : textStart+closeIdx]
		run := &Run{Text: text}

		// Check for run properties (bold, italic, etc.) before this <w:t>
		beforeBlock := xmlBlock[runStart : runStart+tIdx]
		rPrIdx := strings.LastIndex(beforeBlock, "<w:rPr>")
		if rPrIdx >= 0 {
			rPrBlock := beforeBlock[rPrIdx:]
			rp := &RunProps{}
			if strings.Contains(rPrBlock, "<w:b/>") || strings.Contains(rPrBlock, "<w:b>") {
				rp.Bold = true
			}
			if strings.Contains(rPrBlock, "<w:i/>") || strings.Contains(rPrBlock, "<w:i>") {
				rp.Italic = true
			}
			if strings.Contains(rPrBlock, `<w:u w:val="single"/>`) || strings.Contains(rPrBlock, "<w:u>") {
				rp.Underline = true
			}
			// Font size: <w:sz w:val="28"/>
			if szIdx := strings.Index(rPrBlock, `<w:sz w:val="`); szIdx >= 0 {
				szRest := rPrBlock[szIdx+len(`<w:sz w:val="`):]
				szEnd := strings.IndexByte(szRest, '"')
				if szEnd >= 0 {
					if sz, err := strconv.ParseFloat(szRest[:szEnd], 64); err == nil {
						rp.FontSize = sz / 2.0
					}
				}
			}
			// Font name
			if fnIdx := strings.Index(rPrBlock, `w:ascii="`); fnIdx >= 0 {
				fnRest := rPrBlock[fnIdx+len(`w:ascii="`):]
				fnEnd := strings.IndexByte(fnRest, '"')
				if fnEnd >= 0 {
					rp.FontName = fnRest[:fnEnd]
				}
			}
			if rp.FontName == "" {
				if fnIdx := strings.Index(rPrBlock, `w:hAnsi="`); fnIdx >= 0 {
					fnRest := rPrBlock[fnIdx+len(`w:hAnsi="`):]
					fnEnd := strings.IndexByte(fnRest, '"')
					if fnEnd >= 0 {
						rp.FontName = fnRest[:fnEnd]
					}
				}
			}
			// Color: <w:color w:val="FF0000"/>
			if clrIdx := strings.Index(rPrBlock, `<w:color w:val="`); clrIdx >= 0 {
				clrRest := rPrBlock[clrIdx+len(`<w:color w:val="`):]
				clrEnd := strings.IndexByte(clrRest, '"')
				if clrEnd >= 0 {
					rp.Color = "#" + clrRest[:clrEnd]
				}
			}
			run.RPr = rp
		}

		para.Runs = append(para.Runs, run)
		runStart = textStart + closeIdx + len("</w:t>")
	}

	return para
}

// parseTableFromXML parses a raw <w:tbl> XML block into a Table.
func parseTableFromXML(xmlBlock string) (*Table, error) {
	tbl := &Table{}

	// Extract rows
	rowStart := 0
	for {
		trIdx := strings.Index(xmlBlock[rowStart:], "<w:tr>")
		if trIdx < 0 {
			break
		}
		trStart := rowStart + trIdx
		closeIdx := strings.Index(xmlBlock[trStart+5:], "</w:tr>") // +5 for "<w:tr>"
		if closeIdx < 0 {
			break
		}
		trBlock := xmlBlock[trStart : trStart+closeIdx+len("</w:tr>")]

		row := &TableRow{}
		// Extract cells
		cellStart := 0
		for {
			tcIdx := strings.Index(trBlock[cellStart:], "<w:tc>")
			if tcIdx < 0 {
				break
			}
			tcStart := cellStart + tcIdx
			tcClose := strings.Index(trBlock[tcStart+6:], "</w:tc>")
			if tcClose < 0 {
				break
			}
			tcBlock := trBlock[tcStart : tcStart+tcClose+len("</w:tc>")]

			cell := &TableCell{Colspan: 1, Rowspan: 1}

			// Check for gridSpan
			if gsIdx := strings.Index(tcBlock, `w:gridSpan`); gsIdx >= 0 {
				valIdx := strings.Index(tcBlock[gsIdx:], `w:val="`)
				if valIdx >= 0 {
					valStart := gsIdx + valIdx + len(`w:val="`)
					valEnd := strings.IndexByte(tcBlock[valStart:], '"')
					if valEnd >= 0 {
						if v, err := strconv.Atoi(tcBlock[valStart : valStart+valEnd]); err == nil {
							cell.Colspan = v
						}
					}
				}
			}

			// Extract text content from cells
			// Find text content area: skip past </w:tcPr> if present
			contentStart := 0
			if tcPrEnd := strings.Index(tcBlock, "</w:tcPr>"); tcPrEnd >= 0 {
				contentStart = tcPrEnd + len("</w:tcPr>")
			} else if tcPrEnd2 := strings.Index(tcBlock, "</w:tcPr"); tcPrEnd2 >= 0 {
				contentStart = tcPrEnd2 + len("</w:tcPr>")
			}
			// Find <w:t> inside the content area
			txtIdx := strings.Index(tcBlock[contentStart:], "<w:t")
			if txtIdx >= 0 {
				actualIdx := contentStart + txtIdx
				tagEnd := strings.IndexByte(tcBlock[actualIdx:], '>')
				if tagEnd >= 0 {
					textStart := actualIdx + tagEnd + 1
					txtClose := strings.Index(tcBlock[textStart:], "</w:t>")
					if txtClose >= 0 {
						cell.Text = tcBlock[textStart : textStart+txtClose]
					}
				}
			}
			// Collect full multi-paragraph cell text by finding all <w:t> blocks
			{
				var texts []string
				searchStart := contentStart
				for {
					ti := strings.Index(tcBlock[searchStart:], "<w:t")
					if ti < 0 {
						break
					}
					ai := searchStart + ti
					te := strings.IndexByte(tcBlock[ai:], '>')
					if te < 0 {
						break
					}
					ts := ai + te + 1
					tc := strings.Index(tcBlock[ts:], "</w:t>")
					if tc < 0 {
						break
					}
					t := tcBlock[ts : ts+tc]
					if t != "" {
						texts = append(texts, t)
					}
					searchStart = ts + tc + len("</w:t>")
				}
				if len(texts) > 0 {
					cell.Text = strings.Join(texts, " ")
				}
			}

			row.Cells = append(row.Cells, cell)
			cellStart = tcStart + tcClose + len("</w:tc>")
		}

		tbl.Rows = append(tbl.Rows, row)
		rowStart = trStart + closeIdx + len("</w:tr>")
	}

	return tbl, nil
}

// Keep parseParagraph as a helper for non-body contexts (e.g., cell parsing in tables).
// For body-level parsing, we use parseParagraphFromXML instead.
func parseParagraph(dec *xml.Decoder, endTag xml.Name) (*Paragraph, []byte, error) {
	para := &Paragraph{}
	var rawXMLBuf bytes.Buffer
	enc := xml.NewEncoder(&rawXMLBuf)

	depth := 1

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			enc.EncodeToken(t)

		case xml.EndElement:
			depth--
			enc.EncodeToken(t)
			if depth <= 0 && t.Name == endTag {
				enc.Flush()
				return para, rawXMLBuf.Bytes(), nil
			}

		case xml.CharData:
			enc.EncodeToken(t)
		case xml.Comment:
			enc.EncodeToken(t)
		case xml.ProcInst:
			enc.EncodeToken(t)
		case xml.Directive:
			enc.EncodeToken(t)
		}
	}

	enc.Flush()
	return para, rawXMLBuf.Bytes(), nil
}

// parseRun reads a <w:r> element and returns a Run.
func parseRun(dec *xml.Decoder, endTag xml.Name) (*Run, error) {
	run := &Run{}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "rPr" && (t.Name.Space == nsW || t.Name.Space == "") {
				run.RPr = parseRunProps(dec, t.Name)
			} else if t.Name.Local == "t" && (t.Name.Space == nsW || t.Name.Space == "") {
				inner, err := dec.Token()
				if err == nil {
					if cd, ok := inner.(xml.CharData); ok {
						run.Text += string(cd)
					}
				}
			}

		case xml.EndElement:
			if t.Name == endTag {
				return run, nil
			}
		}
	}

	return run, nil
}

// parseRunProps reads <w:rPr> element and extracts character formatting.
func parseRunProps(dec *xml.Decoder, endTag xml.Name) *RunProps {
	rp := &RunProps{}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "b", "bCs":
				// <w:b/> or <w:b w:val="true"/>
				val := attrVal(t.Attr, nsW, "val")
				if val == "" || val == "true" || val == "1" || val == "on" {
					rp.Bold = true
				}
			case "i", "iCs":
				val := attrVal(t.Attr, nsW, "val")
				if val == "" || val == "true" || val == "1" || val == "on" {
					rp.Italic = true
				}
			case "u":
				val := attrVal(t.Attr, nsW, "val")
				if val == "" || val == "single" || val == "true" || val == "1" {
					rp.Underline = true
				}
			case "sz", "szCs":
				val := attrVal(t.Attr, nsW, "val")
				if val != "" {
					if sz, err := strconv.ParseFloat(val, 64); err == nil {
						rp.FontSize = sz / 2.0 // stored in half-points
					}
				}
			case "rFonts":
				rp.FontName = attrVal(t.Attr, nsW, "ascii")
				if rp.FontName == "" {
					rp.FontName = attrVal(t.Attr, nsW, "hAnsi")
				}
				rp.FontNameEastAsia = attrVal(t.Attr, nsW, "eastAsia")
			case "color":
				val := attrVal(t.Attr, nsW, "val")
				if val != "" {
					rp.Color = "#" + val
				}
			case "highlight":
				val := attrVal(t.Attr, nsW, "val")
				if val != "" && rp.Color == "" {
					_ = val
				}
			}

		case xml.EndElement:
			if t.Name == endTag {
				return rp
			}
		}
	}

	return rp
}

// parseParaProps reads <w:pPr> element and extracts paragraph formatting.
func parseParaProps(dec *xml.Decoder, endTag xml.Name) *ParaProps {
	pp := &ParaProps{}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "jc":
				val := attrVal(t.Attr, nsW, "val")
				if val != "" {
					pp.Alignment = val
				}
			case "spacing":
				if before := attrVal(t.Attr, nsW, "before"); before != "" {
					if v, err := strconv.ParseFloat(before, 64); err == nil {
						pp.SpaceBefore = v / 20.0 // twips to points
					}
				}
				if after := attrVal(t.Attr, nsW, "after"); after != "" {
					if v, err := strconv.ParseFloat(after, 64); err == nil {
						pp.SpaceAfter = v / 20.0
					}
				}
				if line := attrVal(t.Attr, nsW, "line"); line != "" {
					if v, err := strconv.ParseFloat(line, 64); err == nil {
						pp.LineSpacing = v / 240.0 // 240ths of a line
					}
				}
			case "ind":
				if left := attrVal(t.Attr, nsW, "left"); left != "" {
					if v, err := strconv.ParseFloat(left, 64); err == nil {
						pp.LeftIndent = v / 567.0 // twips to cm (1cm ≈ 567twips)
					}
				}
				if firstLine := attrVal(t.Attr, nsW, "firstLine"); firstLine != "" {
					if v, err := strconv.ParseFloat(firstLine, 64); err == nil {
						pp.FirstLineIndent = v / 20.0 // twips to points
					}
				}
			}

		case xml.EndElement:
			if t.Name == endTag {
				return pp
			}
		}
	}

	return pp
}

// parseParaPropsInline is like parseParaProps but also extracts pStyle.
func parseParaPropsInline(dec *xml.Decoder, endTag xml.Name) (*ParaProps, error) {
	pp := &ParaProps{}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return pp, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "pStyle":
				// Already parsed at paragraph level, skip
			case "jc":
				val := attrVal(t.Attr, nsW, "val")
				if val != "" {
					pp.Alignment = val
				}
			case "spacing":
				if before := attrVal(t.Attr, nsW, "before"); before != "" {
					if v, err := strconv.ParseFloat(before, 64); err == nil {
						pp.SpaceBefore = v / 20.0
					}
				}
				if after := attrVal(t.Attr, nsW, "after"); after != "" {
					if v, err := strconv.ParseFloat(after, 64); err == nil {
						pp.SpaceAfter = v / 20.0
					}
				}
				if line := attrVal(t.Attr, nsW, "line"); line != "" {
					if v, err := strconv.ParseFloat(line, 64); err == nil {
						pp.LineSpacing = v / 240.0
					}
				}
			case "ind":
				if left := attrVal(t.Attr, nsW, "left"); left != "" {
					if v, err := strconv.ParseFloat(left, 64); err == nil {
						pp.LeftIndent = v / 567.0
					}
				}
				if firstLine := attrVal(t.Attr, nsW, "firstLine"); firstLine != "" {
					if v, err := strconv.ParseFloat(firstLine, 64); err == nil {
						pp.FirstLineIndent = v / 20.0
					}
				}
			}

		case xml.EndElement:
			if t.Name == endTag {
				return pp, nil
			}
		}
	}

	return pp, nil
}

// parseTable reads a <w:tbl> element and returns a Table.
func parseTable(dec *xml.Decoder, endTag xml.Name) (*Table, error) {
	tbl := &Table{}

	depth := 1 // <w:tbl> was already consumed
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "tr" && (t.Name.Space == nsW || t.Name.Space == "") {
				row, err := parseTableRow(dec, t.Name)
				if err != nil {
					return nil, err
				}
				tbl.Rows = append(tbl.Rows, row)
			}

		case xml.EndElement:
			depth--
			if depth <= 0 && t.Name == endTag {
				return tbl, nil
			}
		}
	}

	return tbl, nil
}

// parseTableRow reads a <w:tr> element.
func parseTableRow(dec *xml.Decoder, endTag xml.Name) (*TableRow, error) {
	row := &TableRow{}
	depth := 1 // <w:tr> was already consumed

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "tc" && (t.Name.Space == nsW || t.Name.Space == "") {
				cell, err := parseTableCell(dec, t.Name)
				if err != nil {
					return nil, err
				}
				row.Cells = append(row.Cells, cell)
			}

		case xml.EndElement:
			depth--
			if depth <= 0 && t.Name == endTag {
				return row, nil
			}
		}
	}

	return row, nil
}

// parseTableCell reads a <w:tc> element.
func parseTableCell(dec *xml.Decoder, endTag xml.Name) (*TableCell, error) {
	cell := &TableCell{
		Colspan: 1,
		Rowspan: 1,
	}
	depth := 1 // <w:tc> was already consumed

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "tcPr" && (t.Name.Space == nsW || t.Name.Space == "") {
				// Parse gridSpan and vMerge
				for {
					inner, err := dec.Token()
					if err == io.EOF {
						break
					}
					if err != nil {
						break
					}

					switch it := inner.(type) {
					case xml.StartElement:
						if it.Name.Local == "gridSpan" {
							val := attrVal(it.Attr, nsW, "val")
							if val != "" {
								if v, err := strconv.Atoi(val); err == nil {
									cell.Colspan = v
								}
							}
						} else if it.Name.Local == "vMerge" {
							cell.Rowspan = 2
						}
					case xml.EndElement:
						if it.Name == t.Name {
							goto cellDone
						}
					}
				}
			cellDone:
			} else if t.Name.Local == "p" && (t.Name.Space == nsW || t.Name.Space == "") {
				para, _, err := parseParagraph(dec, t.Name)
				if err == nil {
					if cell.Text == "" {
						cell.Text = para.Text()
					} else {
						cell.Text += "\n" + para.Text()
					}
				}
			}

		case xml.EndElement:
			depth--
			if depth <= 0 && t.Name == endTag {
				return cell, nil
			}
		}
	}

	return cell, nil
}

// Text returns the concatenated text of all runs in the paragraph.
func (p *Paragraph) Text() string {
	if len(p.Runs) == 0 {
		return ""
	}
	var parts []string
	for _, r := range p.Runs {
		if r != nil {
			parts = append(parts, r.Text)
		}
	}
	return strings.Join(parts, "")
}
