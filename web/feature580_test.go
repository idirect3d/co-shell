// Author: L.Shuang
// Created: 2026-10-02
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
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
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package web

import (
	"strings"
	"testing"
)

// FEATURE-580: the export entries of render_ui blocks and windows (Markdown
// download / PDF print) are pure frontend wiring, so the Go side guards the
// structure the browser depends on: the module is embedded and loaded, every
// surface carries its entry, and the print stylesheet keeps the sheet clean.
// The behaviour itself is covered by use-case/FEATURE-580 (browser assertions).
func TestFEATURE580ExportWiring(t *testing.T) {
	export := staticAsset(t, "/static/ui-export.js")
	for _, want := range []string{
		"treeToMarkdown", "printTree", "downloadMarkdown", "attachBlock",
		"ui-print-root", "data-ui-export-item", "data-theme",
	} {
		if !strings.Contains(export, want) {
			t.Errorf("ui-export.js is missing %q", want)
		}
	}
	// The print container must be a direct child of <body>, otherwise the
	// "hide every other body child" print rule cannot isolate it.
	if !strings.Contains(export, "document.body.appendChild(node)") {
		t.Error("ui-export.js does not mount the print container on <body>")
	}

	html := staticAsset(t, "/static/")
	if !strings.Contains(html, `<script src="static/ui-export.js"></script>`) {
		t.Error("index.html does not load ui-export.js")
	}
	// One entry on the float window head, one on the panel head.
	if got := strings.Count(html, `data-ui-export="window"`); got != 2 {
		t.Errorf("index.html window/panel export entries = %d, want 2", got)
	}

	app := staticAsset(t, "/static/app.js")
	if !strings.Contains(app, "UIExport.attachBlock(") {
		t.Error("renderUIBlock does not attach the block export entry")
	}

	windows := staticAsset(t, "/static/ui-windows.js")
	if !strings.Contains(windows, "function exportPayload(") || !strings.Contains(windows, "exportPayload: exportPayload,") {
		t.Error("ui-windows.js does not expose exportPayload to the export module")
	}

	css := staticAsset(t, "/static/style.css")
	if !strings.Contains(css, ".ui-print-root { display: none; }") {
		t.Error("the print container must stay invisible on screen")
	}
	printBlock := between(css, "@media print {", "\n}\n")
	if printBlock == "" {
		t.Fatal("style.css has no @media print block")
	}
	if !strings.Contains(printBlock, "body > *:not(.ui-print-root)") {
		t.Error("the print stylesheet does not hide every other body child")
	}
	if !strings.Contains(printBlock, "@page") {
		t.Error("the print stylesheet does not set the page margins")
	}
}
