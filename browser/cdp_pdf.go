// Author: L.Shuang
// Created: 2026-09-26
// Last Modified: 2026-09-26
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
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// PDF export through the Page domain: Page.printToPDF renders the current page
// and returns base64 data, which is decoded to a file on disk (FEATURE-556
// Phase 3).
//
// Only printBackground is defaulted (to true, because pages usually rely on CSS
// backgrounds); every other option is sent only when the caller sets it, so
// Chrome keeps its own default.

package browser

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// pdfFileNameMaxLength bounds the generated file name.
	pdfFileNameMaxLength = 120
	// PDFDefaultDir is the workspace-relative directory used when the caller
	// does not ask for a path.
	PDFDefaultDir = "download/browser"
)

// PDFOptions controls Page.printToPDF. Nil pointers keep Chrome's default.
type PDFOptions struct {
	Landscape       *bool
	PrintBackground *bool
	PaperWidth      *float64
	PaperHeight     *float64
	Scale           *float64
}

// PrintToPDF renders the current page and returns the base64-encoded PDF data.
func PrintToPDF(ctx context.Context, caller CDPCaller, options PDFOptions) (string, error) {
	result, err := caller.Call(ctx, "Page.printToPDF", PrintToPDFParams(options))
	if err != nil {
		return "", fmt.Errorf("cannot print page to PDF: %w", err)
	}
	data, _ := result["data"].(string)
	if data == "" {
		return "", fmt.Errorf("page produced no PDF data")
	}
	return data, nil
}

// PrintToPDFParams builds the Page.printToPDF parameters.
func PrintToPDFParams(options PDFOptions) map[string]interface{} {
	params := map[string]interface{}{"printBackground": true}

	if options.PrintBackground != nil {
		params["printBackground"] = *options.PrintBackground
	}
	if options.Landscape != nil {
		params["landscape"] = *options.Landscape
	}
	if options.PaperWidth != nil {
		params["paperWidth"] = *options.PaperWidth
	}
	if options.PaperHeight != nil {
		params["paperHeight"] = *options.PaperHeight
	}
	if options.Scale != nil {
		params["scale"] = *options.Scale
	}
	return params
}

// SavePDF decodes base64 PDF data into path (creating parent directories) and
// returns the number of bytes written. Nothing is written when the data is not
// valid base64.
func SavePDF(data, path string) (int, error) {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return 0, fmt.Errorf("cannot decode PDF data: %w", err)
	}
	if len(decoded) == 0 {
		return 0, fmt.Errorf("decoded PDF data is empty")
	}
	directory := filepath.Dir(path)
	if directory != "" && directory != "." {
		if err := os.MkdirAll(directory, 0755); err != nil {
			return 0, fmt.Errorf("cannot create PDF directory: %w", err)
		}
	}
	if err := os.WriteFile(path, decoded, 0644); err != nil {
		return 0, fmt.Errorf("cannot write PDF file: %w", err)
	}
	return len(decoded), nil
}

// PDFFileName builds the file name for a page title: characters that a file
// system rejects are replaced with '-', an empty title falls back to "page",
// and a missing .pdf extension is appended.
func PDFFileName(title string) string {
	name := sanitizeFileName(title)
	if name == "" {
		name = "page"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".pdf") {
		name += ".pdf"
	}
	return name
}

// sanitizeFileName drops characters that are not valid in a file name.
func sanitizeFileName(name string) string {
	var builder strings.Builder
	for _, char := range strings.TrimSpace(name) {
		switch {
		case char < 0x20:
			builder.WriteRune('-')
		case strings.ContainsRune(`/\:*?"<>|`, char):
			builder.WriteRune('-')
		default:
			builder.WriteRune(char)
		}
	}

	out := strings.Trim(builder.String(), " .-")
	if len(out) > pdfFileNameMaxLength {
		out = strings.TrimSpace(out[:pdfFileNameMaxLength])
	}
	return out
}
