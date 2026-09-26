// Integration tests for FEATURE-556 Phase 3 (console collection, file upload,
// PDF export and network observation) against a real headless Chrome.
//
// Use-case: use-case/FEATURE-556/FEATURE-556-UC-0020.md (group B, B-026..B-031).
// The tests are gated by testing.Short() and skipped when no Chrome is
// installed, so `go test ./... -short` stays hermetic. Fixture pages are served
// inline from an httptest server, so no testdata file is needed.

package browser

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestIntegrationP3ConsoleEndToEnd covers B-026: console API calls, an uncaught
// exception and a failed subresource are collected once a tool asked for
// console capture, and capture stays off until then.
func TestIntegrationP3ConsoleEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chrome integration test in -short mode")
	}
	if !IsChromeAvailable() {
		t.Skip("Chrome/Chromium is not installed")
	}

	manager, client := uc556P3Session(t)
	fixtures := uc556P3StartServer(t)

	// Capture is lazy: ordinary browsing must not enable Runtime/Log domains.
	if manager.ConsoleManager() != nil {
		t.Fatalf("console capture started before it was requested")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	console, err := manager.EnsureConsoleCapture(ctx)
	if err != nil {
		t.Fatalf("cannot start console capture: %v", err)
	}
	if manager.ConsoleManager() != console {
		t.Fatalf("ConsoleManager() did not return the started collector")
	}

	uc556Navigate(t, client, fixtures.server.URL+"/console.html")
	if got := uc556Eval(t, client, "document.querySelector('#out').textContent"); got != "ready" {
		t.Fatalf("console page did not finish loading (out=%q)", got)
	}

	var messages []ConsoleMessage
	uc556P3Eventually(t, 15*time.Second, "console levels and the uncaught exception", func() bool {
		messages = append(messages, console.TakeMessages()...)
		return uc556P3HasLevel(messages, "log") &&
			uc556P3HasLevel(messages, "info") &&
			uc556P3HasLevel(messages, "warning") &&
			uc556P3HasLevel(messages, "error") &&
			uc556P3HasText(messages, "p3-uncaught")
	})

	for _, want := range []string{"p3-log", "p3-info", "p3-warn", "p3-error", "p3-uncaught"} {
		if !uc556P3HasText(messages, want) {
			t.Fatalf("console output has no entry containing %q: %v", want, uc556P3Texts(messages))
		}
	}
	if !uc556P3HasText(messages, "404") {
		t.Fatalf("failed subresource was not reported: %v", uc556P3Texts(messages))
	}
	t.Logf("collected %d console messages, received=%d", len(messages), console.Received())
}

// TestIntegrationP3ConsoleSummary covers B-027: the summary clips the message
// count and the length of each line, and draining the buffer empties it.
func TestIntegrationP3ConsoleSummary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chrome integration test in -short mode")
	}
	if !IsChromeAvailable() {
		t.Skip("Chrome/Chromium is not installed")
	}

	manager, client := uc556P3Session(t)
	fixtures := uc556P3StartServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	console, err := manager.EnsureConsoleCapture(ctx)
	if err != nil {
		t.Fatalf("cannot start console capture: %v", err)
	}
	uc556Navigate(t, client, fixtures.server.URL+"/console.html")

	var messages []ConsoleMessage
	uc556P3Eventually(t, 15*time.Second, "at least five console messages", func() bool {
		messages = append(messages, console.TakeMessages()...)
		return len(messages) >= 5 && uc556P3HasText(messages, "p3-uncaught")
	})

	lines := ConsoleSummary(messages, ConsoleSummaryEntries, ConsoleSummaryChars)
	if len(lines) != ConsoleSummaryEntries {
		t.Fatalf("ConsoleSummary returned %d lines, want %d", len(lines), ConsoleSummaryEntries)
	}
	// truncateString keeps maxChars bytes and appends "...", so a clipped line
	// may exceed the configured width by three characters.
	for _, line := range lines {
		if len(line) > ConsoleSummaryChars+3 {
			t.Fatalf("summary line has %d bytes, want at most %d: %q", len(line), ConsoleSummaryChars+3, line)
		}
	}

	// The long message is clipped: its 300 x characters cannot survive.
	long := uc556P3Longest(messages)
	if !strings.Contains(long.Text, "p3-long") {
		t.Fatalf("long console message not collected: %q", long.Text)
	}
	clipped := ConsoleSummary([]ConsoleMessage{long}, 1, 40)
	if len(clipped) != 1 {
		t.Fatalf("ConsoleSummary(long) returned %d lines, want 1", len(clipped))
	}
	if strings.Count(clipped[0], "x") >= 300 {
		t.Fatalf("long message was not truncated: %q", clipped[0])
	}
	if len(clipped[0]) > 43 {
		t.Fatalf("truncated line has %d bytes, want at most 43: %q", len(clipped[0]), clipped[0])
	}

	// Draining is destructive: a message is reported exactly once.
	_ = console.TakeMessages()
	if again := console.TakeMessages(); len(again) != 0 {
		t.Fatalf("second drain returned %d messages, want none", len(again))
	}
	if got := console.Len(); got != 0 {
		t.Fatalf("Len()=%d after draining, want 0", got)
	}
}

// TestIntegrationP3FileUpload covers B-028: one and several files are attached
// to a file input, and bad input is reported instead of silently ignored.
func TestIntegrationP3FileUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chrome integration test in -short mode")
	}
	if !IsChromeAvailable() {
		t.Skip("Chrome/Chromium is not installed")
	}

	_, client := uc556P3Session(t)
	fixtures := uc556P3StartServer(t)
	uc556Navigate(t, client, fixtures.server.URL+"/upload.html")

	directory := t.TempDir()
	first := filepath.Join(directory, "first.txt")
	second := filepath.Join(directory, "second.txt")
	for _, file := range []string{first, second} {
		if err := os.WriteFile(file, []byte("uc556-upload\n"), 0644); err != nil {
			t.Fatalf("cannot write %s: %v", file, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := UploadFiles(ctx, client, "#files", []string{first}); err != nil {
		t.Fatalf("UploadFiles(one file): %v", err)
	}
	if got := uc556Eval(t, client, "document.querySelector('#files').files.length"); got != "1" {
		t.Fatalf("after one upload files.length=%s, want 1", got)
	}
	if got := uc556Eval(t, client, "document.querySelector('#files').files[0].name"); got != "first.txt" {
		t.Fatalf("uploaded file name=%q, want first.txt", got)
	}

	if err := UploadFiles(ctx, client, "#files", []string{first, second}); err != nil {
		t.Fatalf("UploadFiles(two files): %v", err)
	}
	if got := uc556Eval(t, client, "document.querySelector('#files').files.length"); got != "2" {
		t.Fatalf("after two uploads files.length=%s, want 2", got)
	}

	// The agent resolves upload paths inside the workspace first, so a missing
	// or escaping path is rejected before any CDP call is made.
	if _, err := WorkspaceFiles(directory, []string{"absent.txt"}); err == nil {
		t.Fatalf("WorkspaceFiles(missing file) returned no error")
	}
	if _, err := WorkspaceFiles(directory, []string{"../outside.txt"}); err == nil {
		t.Fatalf("WorkspaceFiles(path outside the workspace) returned no error")
	}

	// An unmatched selector is reported by UploadFiles itself, and the input
	// keeps the files it already had.
	if err := UploadFiles(ctx, client, "#absent", []string{first}); err == nil {
		t.Fatalf("UploadFiles(unknown selector) returned no error")
	}
	if got := uc556Eval(t, client, "document.querySelector('#files').files.length"); got != "2" {
		t.Fatalf("files.length=%s after a failed upload, want the previous 2", got)
	}
}

// TestIntegrationP3PDFExport covers B-029: the page is rendered to a real PDF
// file under the default directory, and an explicit landscape render works too.
func TestIntegrationP3PDFExport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chrome integration test in -short mode")
	}
	if !IsChromeAvailable() {
		t.Skip("Chrome/Chromium is not installed")
	}

	_, client := uc556P3Session(t)
	fixtures := uc556P3StartServer(t)
	uc556Navigate(t, client, fixtures.server.URL+"/plain.html")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, err := PrintToPDF(ctx, client, PDFOptions{})
	if err != nil {
		t.Fatalf("PrintToPDF: %v", err)
	}

	path := filepath.Join(t.TempDir(), PDFDefaultDir, PDFFileName("uc556-pdf"))
	size, err := SavePDF(data, path)
	if err != nil {
		t.Fatalf("SavePDF: %v", err)
	}
	if size == 0 {
		t.Fatalf("SavePDF wrote no bytes")
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read the written PDF: %v", err)
	}
	if len(written) != size {
		t.Fatalf("written size=%d, SavePDF reported %d", len(written), size)
	}
	if !bytes.HasPrefix(written, []byte("%PDF")) {
		t.Fatalf("written file does not start with %%PDF: %q", written[:min(8, len(written))])
	}
	if PDFFileName("uc556-pdf") != "uc556-pdf.pdf" {
		t.Fatalf("PDFFileName returned %q, want uc556-pdf.pdf", PDFFileName("uc556-pdf"))
	}

	landscape := true
	width, height := 8.27, 11.69
	landscapeData, err := PrintToPDF(ctx, client, PDFOptions{
		Landscape:   &landscape,
		PaperWidth:  &width,
		PaperHeight: &height,
	})
	if err != nil {
		t.Fatalf("PrintToPDF(landscape): %v", err)
	}
	if len(landscapeData) == 0 {
		t.Fatalf("PrintToPDF(landscape) returned no data")
	}
}

// TestIntegrationP3NetworkObservation covers B-030/B-031: request outcomes are
// observed from before the navigation, failures are distinguishable from 404
// responses, and only_failures/limit narrow the snapshot non-destructively.
func TestIntegrationP3NetworkObservation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-Chrome integration test in -short mode")
	}
	if !IsChromeAvailable() {
		t.Skip("Chrome/Chromium is not installed")
	}

	manager, client := uc556P3Session(t)
	fixtures := uc556P3StartServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if manager.NetworkCollector() != nil {
		t.Fatalf("network observation started before it was requested")
	}
	collector, err := manager.EnsureNetworkCollector(ctx)
	if err != nil {
		t.Fatalf("cannot start network observation: %v", err)
	}

	// CDP never replays network events, so the collector must exist before the
	// traffic it should observe.
	uc556Navigate(t, client, fixtures.server.URL+"/network.html")

	uc556P3Eventually(t, 20*time.Second, "successful, 404 and failed requests", func() bool {
		return uc556P3FindEntry(collector, "/real.png") != nil && uc556P3FindEntry(collector, "unreachable.png") != nil
	})

	entries := collector.Entries(false, 0)
	ok := uc556P3FindEntry(collector, "/real.png")
	if ok == nil {
		t.Fatalf("no entry for /real.png: %v", uc556P3URLs(entries))
	}
	if ok.Status != 200 {
		t.Fatalf("/real.png status=%d, want 200", ok.Status)
	}
	if !strings.Contains(ok.MIMEType, "image/png") {
		t.Fatalf("/real.png mimeType=%q, want image/png", ok.MIMEType)
	}

	// A 404 is a response, not a transport failure.
	gone := uc556P3FindEntry(collector, "/missing-404.png")
	if gone == nil {
		t.Fatalf("no entry for /missing-404.png: %v", uc556P3URLs(entries))
	}
	if gone.Status != 404 || gone.Failed {
		t.Fatalf("/missing-404.png status=%d failed=%v, want 404/false", gone.Status, gone.Failed)
	}

	// The closed port fails at the transport level, with the reason recorded.
	bad := uc556P3FindEntry(collector, "unreachable.png")
	if bad == nil || !bad.Failed || bad.ErrorText == "" {
		t.Fatalf("unreachable image not reported as failed: %+v", bad)
	}

	// The document request is part of the same snapshot.
	if uc556P3FindEntry(collector, "/network.html") == nil {
		t.Fatalf("document request missing from the snapshot: %v", uc556P3URLs(entries))
	}

	failures := collector.Entries(true, 0)
	if len(failures) == 0 {
		t.Fatalf("only_failures returned no entry")
	}
	for _, entry := range failures {
		if !entry.Failed {
			t.Fatalf("only_failures returned a successful request: %+v", entry)
		}
	}
	if limited := collector.Entries(false, 2); len(limited) != 2 {
		t.Fatalf("limit=2 returned %d entries, want 2", len(limited))
	}

	// Reading never shrinks the buffer.
	if after := collector.Count(); after < len(entries) {
		t.Fatalf("Count()=%d shrank after reading %d entries", after, len(entries))
	}

	lines, remaining := NetworkFailureLines(failures, NetworkSummaryEntries, NetworkSummaryChars)
	if len(lines) == 0 {
		t.Fatalf("NetworkFailureLines returned no line")
	}
	if remaining < 0 {
		t.Fatalf("NetworkFailureLines reported remaining=%d", remaining)
	}
	t.Logf("observed %d requests, %d failures: %v", collector.Received(), len(failures), lines)
}

// uc556P3FindEntry returns the first observed request whose URL contains part.
func uc556P3FindEntry(collector *NetworkCollector, part string) *NetworkEntry {
	entries := collector.Entries(false, 0)
	for i := range entries {
		if strings.Contains(entries[i].URL, part) {
			return &entries[i]
		}
	}
	return nil
}

// uc556P3URLs renders the observed URLs for failure messages.
func uc556P3URLs(entries []NetworkEntry) []string {
	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		urls = append(urls, entry.URL)
	}
	return urls
}

// uc556P3HasLevel reports whether any message has the given CDP level.
func uc556P3HasLevel(messages []ConsoleMessage, level string) bool {
	for _, message := range messages {
		if message.Level == level {
			return true
		}
	}
	return false
}

// uc556P3HasText reports whether any message contains part.
func uc556P3HasText(messages []ConsoleMessage, part string) bool {
	for _, message := range messages {
		if strings.Contains(message.Text, part) {
			return true
		}
	}
	return false
}

// uc556P3Texts renders the collected message texts for failure messages.
func uc556P3Texts(messages []ConsoleMessage) []string {
	texts := make([]string, 0, len(messages))
	for _, message := range messages {
		texts = append(texts, fmt.Sprintf("[%s/%s] %s", message.Level, message.Source, message.Text))
	}
	return texts
}

// uc556P3Longest returns the message with the longest text.
func uc556P3Longest(messages []ConsoleMessage) ConsoleMessage {
	longest := ConsoleMessage{}
	for _, message := range messages {
		if len(message.Text) > len(longest.Text) {
			longest = message
		}
	}
	return longest
}
