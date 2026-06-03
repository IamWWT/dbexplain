// Package output provides output capture and context writing utilities.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/IamWWT/dbexplain/internal/analyze"
	ctxcompress "github.com/IamWWT/dbexplain/internal/context"
	"github.com/IamWWT/dbexplain/internal/render"
)

// WriteContext writes AI context files (summary, topology, diagnostics, chunks) to a directory.
func WriteContext(dir string, result *analyze.Result) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("create context dir: %v", err)
	}

	// summary.json
	summary := ctxcompress.GenerateSummary(result, 10)
	writeJSON(filepath.Join(dir, "summary.json"), summary)

	// topology.json
	topo := ctxcompress.GenerateTopology(result)
	writeJSON(filepath.Join(dir, "topology.json"), topo)

	// diagnostics.json
	diag := ctxcompress.GenerateDiagnostics(result.Issues)
	writeJSON(filepath.Join(dir, "diagnostics.json"), diag)

	// retrieval chunks
	chunksDir := filepath.Join(dir, "chunks")
	if err := os.MkdirAll(chunksDir, 0755); err != nil {
		log.Fatalf("create chunks dir: %v", err)
	}
	chunks := ctxcompress.GenerateChunks(result, 15)
	for _, chunk := range chunks {
		md := ctxcompress.RenderChunkMarkdown(&chunk)
		name := strings.ReplaceAll(strings.ReplaceAll(chunk.Table, "/", "_"), "\\", "_") + ".md"
		if err := os.WriteFile(filepath.Join(chunksDir, name), []byte(md), 0644); err != nil {
			log.Printf("write chunk %s: %v", name, err)
		}
	}

	fmt.Fprintf(os.Stderr, "Context written to %s (%d files)\n", dir, 3+len(chunks))
}

func writeJSON(path string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Printf("marshal %s: %v", path, err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("write %s: %v", path, err)
	}
}

// CaptureText captures render.Print output as a string.
func CaptureText(result *analyze.Result, human bool) string {
	r, w, err := os.Pipe()
	if err != nil {
		return ""
	}
	old := os.Stdout
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC] CaptureText: %v", r)
			}
		}()
		io.Copy(&buf, r)
		close(done)
	}()

	render.Print(result, human)
	w.Close()
	os.Stdout = old
	<-done
	return buf.String()
}

// CaptureJSON captures render.PrintJSON output as a string.
func CaptureJSON(result *analyze.Result) string {
	r, w, err := os.Pipe()
	if err != nil {
		return ""
	}
	old := os.Stdout
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC] CaptureJSON: %v", r)
			}
		}()
		io.Copy(&buf, r)
		close(done)
	}()

	render.PrintJSON(result)
	w.Close()
	os.Stdout = old
	<-done
	return buf.String()
}
