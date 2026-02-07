//go:build !windows
// +build !windows

// 指示: miu200521358
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOptionsWithFlags(t *testing.T) {
	errBuf := bytes.NewBuffer(nil)
	opts, err := parseOptions([]string{"-in", "avatar.vrm", "-out", "avatar.pmx"}, errBuf)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if opts.inputPath != "avatar.vrm" {
		t.Fatalf("inputPath mismatch: %s", opts.inputPath)
	}
	if opts.outputPath != "avatar.pmx" {
		t.Fatalf("outputPath mismatch: %s", opts.outputPath)
	}
}

func TestParseOptionsWithPositionals(t *testing.T) {
	errBuf := bytes.NewBuffer(nil)
	opts, err := parseOptions([]string{"avatar.vrm", "result.pmx"}, errBuf)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if opts.inputPath != "avatar.vrm" {
		t.Fatalf("inputPath mismatch: %s", opts.inputPath)
	}
	if opts.outputPath != "result.pmx" {
		t.Fatalf("outputPath mismatch: %s", opts.outputPath)
	}
}

func TestParseOptionsRequireVrmExt(t *testing.T) {
	errBuf := bytes.NewBuffer(nil)
	_, err := parseOptions([]string{"-in", "avatar.pmx"}, errBuf)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), ".vrm") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveOutputPathDefault(t *testing.T) {
	out, err := resolveOutputPath(filepath.Join("work", "avatar.vrm"), "")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	expected := filepath.Join("work", "avatar.pmx")
	if out != expected {
		t.Fatalf("output mismatch: %s != %s", out, expected)
	}
}

func TestResolveOutputPathRequirePmxExt(t *testing.T) {
	_, err := resolveOutputPath("avatar.vrm", "avatar.vmd")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), ".pmx") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunConvertsVrmToPmx(t *testing.T) {
	tempDir := t.TempDir()
	inPath := filepath.Join(tempDir, "avatar.vrm")
	outPath := filepath.Join(tempDir, "avatar.pmx")
	writeTestGLB(t, inPath, map[string]any{
		"asset": map[string]any{
			"version": "2.0",
		},
		"extensionsUsed": []string{"VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name":        "hips_node",
				"translation": []float64{0, 0.8, 0},
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips": map[string]any{"node": 0},
					},
				},
			},
		},
	})

	outBuf := bytes.NewBuffer(nil)
	errBuf := bytes.NewBuffer(nil)
	if err := run([]string{"-in", inPath, "-out", outPath}, outBuf, errBuf); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("output not found: %v", err)
	}
	if info.Size() <= 0 {
		t.Fatalf("output size is invalid: %d", info.Size())
	}
}

// writeTestGLB はテスト用JSONをGLB形式で保存する。
func writeTestGLB(t *testing.T, path string, doc map[string]any) {
	t.Helper()
	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	padding := (4 - (len(jsonBytes) % 4)) % 4
	if padding > 0 {
		jsonBytes = append(jsonBytes, bytes.Repeat([]byte(" "), padding)...)
	}

	totalLength := uint32(12 + 8 + len(jsonBytes))
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(0x46546C67)); err != nil {
		t.Fatalf("write magic failed: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(2)); err != nil {
		t.Fatalf("write version failed: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, totalLength); err != nil {
		t.Fatalf("write total length failed: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(jsonBytes))); err != nil {
		t.Fatalf("write chunk length failed: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(0x4E4F534A)); err != nil {
		t.Fatalf("write chunk type failed: %v", err)
	}
	if _, err := buf.Write(jsonBytes); err != nil {
		t.Fatalf("write chunk body failed: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write glb file failed: %v", err)
	}
}
