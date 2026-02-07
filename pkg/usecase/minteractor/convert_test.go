// 指示: miu200521358
package minteractor

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/adapter/io_model"
)

func TestVrm2PmxUsecaseConvert(t *testing.T) {
	tempDir := t.TempDir()
	inPath := filepath.Join(tempDir, "sample.vrm")
	outPath := filepath.Join(tempDir, "sample.pmx")
	writeGLBForUsecaseTest(t, inPath, map[string]any{
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

	uc := NewVrm2PmxUsecase(Vrm2PmxUsecaseDeps{
		ModelReader: io_model.NewModelRepository(),
		ModelWriter: io_model.NewModelRepository(),
	})

	result, err := uc.Convert(ConvertRequest{InputPath: inPath, OutputPath: outPath})
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if result == nil || result.Model == nil {
		t.Fatalf("result/model is nil")
	}
	if result.Model.VrmData == nil {
		t.Fatalf("vrm data is nil")
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("output not found: %v", err)
	}
}

func TestVrm2PmxUsecaseConvertRequiresPmxExt(t *testing.T) {
	uc := NewVrm2PmxUsecase(Vrm2PmxUsecaseDeps{})
	_, err := uc.Convert(ConvertRequest{InputPath: "sample.vrm", OutputPath: "sample.vmd"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

// writeGLBForUsecaseTest はテスト用JSONをGLB形式で保存する。
func writeGLBForUsecaseTest(t *testing.T, path string, doc map[string]any) {
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
