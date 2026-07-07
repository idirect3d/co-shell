// Author: L.Shuang
// Created: 2026-07-07
// Last Modified: 2026-07-07
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO
// EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES
// OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

package xlsx

import "fmt"

// CellStyle brings together all style components for a cell.
type CellStyle struct {
	Font         *FontStyle      `json:"font,omitempty"`
	Fill         *FillStyle      `json:"fill,omitempty"`
	Border       *BorderStyle    `json:"border,omitempty"`
	Alignment    *AlignmentStyle `json:"alignment,omitempty"`
	NumberFormat string          `json:"number_format,omitempty"` // e.g. "0.00", "#,##0"
}

// FontStyle defines font properties.
type FontStyle struct {
	Name      string `json:"name,omitempty"` // e.g. "Calibri", "微软雅黑"
	Size      int    `json:"size,omitempty"` // font size in points, default 11
	Bold      bool   `json:"bold,omitempty"`
	Italic    bool   `json:"italic,omitempty"`
	Underline bool   `json:"underline,omitempty"`
	Strike    bool   `json:"strike,omitempty"`
	Color     string `json:"color,omitempty"` // e.g. "#FF0000"
}

// FillStyle defines cell background.
type FillStyle struct {
	PatternType string `json:"pattern_type,omitempty"` // "none", "solid" (default)
	FgColor     string `json:"fg_color,omitempty"`     // e.g. "#FFFFCC"
}

// BorderEdge defines one side of the border.
type BorderEdge struct {
	Style string `json:"style,omitempty"` // "thin","medium","thick","dashed","dotted","double"
	Color string `json:"color,omitempty"` // e.g. "#000000"
}

// BorderStyle defines all four sides.
type BorderStyle struct {
	Top    *BorderEdge `json:"top,omitempty"`
	Bottom *BorderEdge `json:"bottom,omitempty"`
	Left   *BorderEdge `json:"left,omitempty"`
	Right  *BorderEdge `json:"right,omitempty"`
}

// AlignmentStyle defines cell alignment and text wrapping.
type AlignmentStyle struct {
	Horizontal   string `json:"horizontal,omitempty"` // "left","center","right","fill","justify"
	Vertical     string `json:"vertical,omitempty"`   // "top","center","bottom","justify"
	WrapText     bool   `json:"wrap_text,omitempty"`
	TextRotation int    `json:"text_rotation,omitempty"` // 0-180 degrees
}

// borderQuad holds 4 border edges as a flat slice for lookup.
type borderQuad []*xlBorderEdge

// styleManager manages a set of styles (xf entries) for a workbook.
type styleManager struct {
	Fonts   []xlFont
	Fills   []xlFill
	Quads   []borderQuad // each quad has 4 edges [top, bottom, left, right]
	Aligns  []xlAlignment
	NumFmts []string
	XFList  []xfEntry
	xfCache map[string]int
}

type xlFont struct {
	Name      string
	Size      int
	Bold      bool
	Italic    bool
	Underline bool
	Strike    bool
	Color     string
}

type xlFill struct {
	Pattern string
	Color   string
}

type xlBorderEdge struct {
	Style string
	Color string
}

type xlAlignment struct {
	Horizontal string
	Vertical   string
	WrapText   bool
	Rotation   int
}

type xfEntry struct {
	FontID   int
	FillID   int
	BorderID int
	AlignID  int
	NumFmtID int
}

// newStyleManager creates a style manager with default entries.
func newStyleManager() *styleManager {
	sm := &styleManager{
		xfCache: make(map[string]int),
	}
	sm.Fonts = append(sm.Fonts, xlFont{Name: "Calibri", Size: 11})
	sm.Fills = append(sm.Fills, xlFill{Pattern: "none"})
	sm.Fills = append(sm.Fills, xlFill{Pattern: "gray125"})
	sm.Quads = append(sm.Quads, borderQuad{nil, nil, nil, nil})
	sm.Aligns = append(sm.Aligns, xlAlignment{})
	sm.XFList = append(sm.XFList, xfEntry{
		FontID: 0, FillID: 0, BorderID: 0, AlignID: 0, NumFmtID: -1,
	})
	return sm
}

// addStyle registers a style and returns its xf index.
func (sm *styleManager) addStyle(cs CellStyle) int {
	key := styleKey(cs)
	if idx, ok := sm.xfCache[key]; ok {
		return idx
	}

	fontID := 0
	if cs.Font != nil {
		fontID = sm.findOrAddFont(*cs.Font)
	}
	fillID := 0
	if cs.Fill != nil {
		fillID = sm.findOrAddFill(*cs.Fill)
	}
	borderID := 0
	if cs.Border != nil {
		borderID = sm.findOrAddBorder(*cs.Border)
	}
	alignID := -1
	if cs.Alignment != nil {
		alignID = sm.findOrAddAlign(*cs.Alignment)
	}
	numFmtID := -1
	if cs.NumberFormat != "" {
		numFmtID = sm.findOrAddNumFmt(cs.NumberFormat)
	}

	idx := len(sm.XFList)
	sm.XFList = append(sm.XFList, xfEntry{
		FontID: fontID, FillID: fillID, BorderID: borderID,
		AlignID: alignID, NumFmtID: numFmtID,
	})
	sm.xfCache[key] = idx
	return idx
}

func styleKey(cs CellStyle) string {
	k := ""
	if cs.Font != nil {
		k += fmt.Sprintf("F:%s/%d/%v/%v/%v/%v/%s;",
			cs.Font.Name, cs.Font.Size, cs.Font.Bold, cs.Font.Italic,
			cs.Font.Underline, cs.Font.Strike, cs.Font.Color)
	}
	if cs.Fill != nil {
		k += fmt.Sprintf("FL:%s/%s;", cs.Fill.PatternType, cs.Fill.FgColor)
	}
	if cs.Border != nil {
		be := func(e *BorderEdge) string {
			if e == nil {
				return "-"
			}
			return e.Style + ":" + e.Color
		}
		k += fmt.Sprintf("B:%s/%s/%s/%s;",
			be(cs.Border.Top), be(cs.Border.Bottom),
			be(cs.Border.Left), be(cs.Border.Right))
	}
	if cs.Alignment != nil {
		k += fmt.Sprintf("A:%s/%s/%v/%d;", cs.Alignment.Horizontal,
			cs.Alignment.Vertical, cs.Alignment.WrapText, cs.Alignment.TextRotation)
	}
	if cs.NumberFormat != "" {
		k += "NF:" + cs.NumberFormat
	}
	return k
}

func edgeToXl(e *BorderEdge) *xlBorderEdge {
	if e == nil {
		return nil
	}
	return &xlBorderEdge{Style: e.Style, Color: e.Color}
}

func xlEdgeEq(a, b *xlBorderEdge) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Style == b.Style && a.Color == b.Color
}

func (sm *styleManager) findOrAddFont(f FontStyle) int {
	for i, e := range sm.Fonts {
		if e.Name == f.Name && e.Size == f.Size &&
			e.Bold == f.Bold && e.Italic == f.Italic &&
			e.Underline == f.Underline && e.Strike == f.Strike &&
			e.Color == f.Color {
			return i
		}
	}
	idx := len(sm.Fonts)
	sm.Fonts = append(sm.Fonts, xlFont{
		Name: f.Name, Size: f.Size,
		Bold: f.Bold, Italic: f.Italic,
		Underline: f.Underline, Strike: f.Strike,
		Color: f.Color,
	})
	return idx
}

func (sm *styleManager) findOrAddFill(f FillStyle) int {
	if f.PatternType == "" {
		f.PatternType = "solid"
	}
	for i, e := range sm.Fills {
		if e.Pattern == f.PatternType && e.Color == f.FgColor {
			return i
		}
	}
	idx := len(sm.Fills)
	sm.Fills = append(sm.Fills, xlFill{Pattern: f.PatternType, Color: f.FgColor})
	return idx
}

func (sm *styleManager) findOrAddBorder(b BorderStyle) int {
	q := borderQuad{edgeToXl(b.Top), edgeToXl(b.Bottom), edgeToXl(b.Left), edgeToXl(b.Right)}
	for i, existing := range sm.Quads {
		if xlEdgeEq(existing[0], q[0]) &&
			xlEdgeEq(existing[1], q[1]) &&
			xlEdgeEq(existing[2], q[2]) &&
			xlEdgeEq(existing[3], q[3]) {
			return i
		}
	}
	idx := len(sm.Quads)
	sm.Quads = append(sm.Quads, q)
	return idx
}

func (sm *styleManager) findOrAddAlign(a AlignmentStyle) int {
	for i, e := range sm.Aligns {
		if e.Horizontal == a.Horizontal && e.Vertical == a.Vertical &&
			e.WrapText == a.WrapText && e.Rotation == a.TextRotation {
			return i
		}
	}
	idx := len(sm.Aligns)
	sm.Aligns = append(sm.Aligns, xlAlignment{
		Horizontal: a.Horizontal, Vertical: a.Vertical,
		WrapText: a.WrapText, Rotation: a.TextRotation,
	})
	return idx
}

// StyleFromXF reconstructs a CellStyle from an xf index, for merge purposes.
// Returns a zero CellStyle if xfIdx is out of range or is the default xf (0).
func (sm *styleManager) StyleFromXF(xfIdx int) CellStyle {
	var cs CellStyle
	if xfIdx < 0 || xfIdx >= len(sm.XFList) {
		return cs
	}
	xf := sm.XFList[xfIdx]
	if xf.FontID >= 0 && xf.FontID < len(sm.Fonts) {
		f := sm.Fonts[xf.FontID]
		cs.Font = &FontStyle{
			Name: f.Name, Size: f.Size,
			Bold: f.Bold, Italic: f.Italic,
			Underline: f.Underline, Strike: f.Strike,
			Color: f.Color,
		}
	}
	if xf.FillID >= 0 && xf.FillID < len(sm.Fills) {
		f := sm.Fills[xf.FillID]
		if f.Pattern != "none" && f.Pattern != "gray125" {
			cs.Fill = &FillStyle{PatternType: f.Pattern, FgColor: f.Color}
		}
	}
	if xf.BorderID >= 0 && xf.BorderID*4+3 < len(sm.Quads) {
		q := sm.Quads[xf.BorderID]
		bs := &BorderStyle{}
		getEdge := func(e *xlBorderEdge) *BorderEdge {
			if e == nil {
				return nil
			}
			return &BorderEdge{Style: e.Style, Color: e.Color}
		}
		bs.Top = getEdge(q[2])
		bs.Bottom = getEdge(q[3])
		bs.Left = getEdge(q[0])
		bs.Right = getEdge(q[1])
		if bs.Top != nil || bs.Bottom != nil || bs.Left != nil || bs.Right != nil {
			cs.Border = bs
		}
	}
	if xf.AlignID >= 0 && xf.AlignID < len(sm.Aligns) {
		a := sm.Aligns[xf.AlignID]
		if a.Horizontal != "" || a.Vertical != "" || a.WrapText || a.Rotation > 0 {
			cs.Alignment = &AlignmentStyle{
				Horizontal: a.Horizontal, Vertical: a.Vertical,
				WrapText: a.WrapText, TextRotation: a.Rotation,
			}
		}
	}
	if xf.NumFmtID >= 0 && xf.NumFmtID < len(sm.NumFmts) {
		cs.NumberFormat = sm.NumFmts[xf.NumFmtID]
	}
	return cs
}

func (sm *styleManager) findOrAddNumFmt(f string) int {
	for i, e := range sm.NumFmts {
		if e == f {
			return i
		}
	}
	idx := len(sm.NumFmts)
	sm.NumFmts = append(sm.NumFmts, f)
	return idx
}
