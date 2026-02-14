// 指示: miu200521358
package minteractor

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/adapter/io_model/pmx"
	"github.com/miu200521358/mu_vrm2pmx/pkg/adapter/io_model/vrm"
)

func TestVrm2PmxUsecasePrepareModelForOutputDoesNotSavePmx(t *testing.T) {
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
	}, nil)

	uc := NewVrm2PmxUsecase(Vrm2PmxUsecaseDeps{
		ModelReader: vrm.NewVrmRepository(),
		ModelWriter: pmx.NewPmxRepository(),
	})

	result, err := uc.PrepareModel(ConvertRequest{InputPath: inPath, OutputPath: outPath})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if result == nil || result.Model == nil {
		t.Fatalf("result/model is nil")
	}
	if result.OutputPath != outPath {
		t.Fatalf("output path mismatch: got=%s want=%s", result.OutputPath, outPath)
	}
	if result.Model.Path() != outPath {
		t.Fatalf("model path mismatch: got=%s want=%s", result.Model.Path(), outPath)
	}

	_, statErr := os.Stat(outPath)
	if statErr == nil || !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("pmx file should not be saved in prepare phase: %v", statErr)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(outPath), "tex")); err != nil {
		t.Fatalf("tex directory not found: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(outPath), "glTF")); err != nil {
		t.Fatalf("glTF directory not found: %v", err)
	}
}

func TestVrm2PmxUsecaseSaveModelAfterPrepare(t *testing.T) {
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
	}, nil)

	uc := NewVrm2PmxUsecase(Vrm2PmxUsecaseDeps{
		ModelReader: vrm.NewVrmRepository(),
		ModelWriter: pmx.NewPmxRepository(),
	})

	result, err := uc.PrepareModel(ConvertRequest{InputPath: inPath, OutputPath: outPath})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if result == nil || result.Model == nil {
		t.Fatalf("result/model is nil")
	}
	if result.Model.VrmData == nil {
		t.Fatalf("vrm data is nil")
	}
	if err := uc.SaveModel(nil, result.OutputPath, result.Model, SaveOptions{}); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("output not found: %v", err)
	}
}

func TestVrm2PmxUsecasePrepareModelReportsBoneMappingAfterReorder(t *testing.T) {
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
				"name":        "hips",
				"translation": []float64{0, 0.8, 0},
				"children":    []int{1},
			},
			map[string]any{
				"name":        "spine",
				"translation": []float64{0, 0.2, 0},
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips":  map[string]any{"node": 0},
						"spine": map[string]any{"node": 1},
					},
				},
			},
		},
	}, nil)

	uc := NewVrm2PmxUsecase(Vrm2PmxUsecaseDeps{
		ModelReader: vrm.NewVrmRepository(),
		ModelWriter: pmx.NewPmxRepository(),
	})
	reporter := &prepareProgressEventCollector{}

	result, err := uc.PrepareModel(ConvertRequest{
		InputPath:        inPath,
		OutputPath:       outPath,
		ProgressReporter: reporter,
	})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if result == nil || result.Model == nil {
		t.Fatalf("result/model is nil")
	}
	if _, err := result.Model.Bones.GetByName("下半身"); err != nil {
		t.Fatalf("expected mapped bone 下半身: %v", err)
	}
	if _, err := result.Model.Bones.GetByName("上半身"); err != nil {
		t.Fatalf("expected mapped bone 上半身: %v", err)
	}

	reorderIdx := reporter.findIndex(PrepareProgressEventTypeReorderCompleted)
	if reorderIdx < 0 {
		t.Fatalf("reorder completed event not reported")
	}
	boneMappingIdx := reporter.findIndex(PrepareProgressEventTypeBoneMappingCompleted)
	if boneMappingIdx < 0 {
		t.Fatalf("bone mapping completed event not reported")
	}
	if reorderIdx >= boneMappingIdx {
		t.Fatalf("expected reorder event before bone mapping event: reorder=%d bone=%d", reorderIdx, boneMappingIdx)
	}
	astanceIdx := reporter.findIndex(PrepareProgressEventTypeAstanceCompleted)
	if astanceIdx < 0 {
		t.Fatalf("a-stance completed event not reported")
	}
	if boneMappingIdx >= astanceIdx {
		t.Fatalf("expected bone mapping event before a-stance event: bone=%d astance=%d", boneMappingIdx, astanceIdx)
	}
}

func TestVrm2PmxUsecasePrepareModelForOutputRequiresPmxExt(t *testing.T) {
	uc := NewVrm2PmxUsecase(Vrm2PmxUsecaseDeps{})
	_, err := uc.PrepareModel(ConvertRequest{InputPath: "sample.vrm", OutputPath: "sample.vmd"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestVrm2PmxUsecasePrepareModelForOutputAutoOutputPathUsesUtility(t *testing.T) {
	tempDir := t.TempDir()
	inPath := filepath.Join(tempDir, "sample.vrm")
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
	}, nil)

	uc := NewVrm2PmxUsecase(Vrm2PmxUsecaseDeps{
		ModelReader: vrm.NewVrmRepository(),
		ModelWriter: pmx.NewPmxRepository(),
	})

	result, err := uc.PrepareModel(ConvertRequest{InputPath: inPath})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if result == nil || result.OutputPath == "" {
		t.Fatalf("output path is empty")
	}
	if filepath.Ext(result.OutputPath) != ".pmx" {
		t.Fatalf("output extension is not pmx: %s", result.OutputPath)
	}
	baseName := filepath.Base(result.OutputPath)
	if baseName != "sample.pmx" {
		t.Fatalf("output file name mismatch: %s", baseName)
	}
	dirName := filepath.Base(filepath.Dir(result.OutputPath))
	matched, matchErr := regexp.MatchString(`^sample_\d{14}$`, dirName)
	if matchErr != nil {
		t.Fatalf("pattern error: %v", matchErr)
	}
	if !matched {
		t.Fatalf("output directory format mismatch: %s", dirName)
	}
	_, statErr := os.Stat(result.OutputPath)
	if statErr == nil || !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("pmx file should not be saved in prepare phase: %v", statErr)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(result.OutputPath), "tex")); err != nil {
		t.Fatalf("tex directory not found: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(result.OutputPath), "glTF")); err != nil {
		t.Fatalf("glTF directory not found: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(result.OutputPath), "glTF", "sample.gltf")); err != nil {
		t.Fatalf("gltf output not found: %v", err)
	}
}

func TestVrm2PmxUsecasePrepareModelForOutputExtractsEmbeddedTexture(t *testing.T) {
	tempDir := t.TempDir()
	inPath := filepath.Join(tempDir, "sample.vrm")
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x63, 0x60, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x01, 0xE5, 0x27, 0xD4, 0xA2, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}
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
		"buffers": []any{
			map[string]any{
				"byteLength": len(pngData),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": 0,
				"byteLength": len(pngData),
			},
		},
		"images": []any{
			map[string]any{
				"name":       "face",
				"bufferView": 0,
				"mimeType":   "image/png",
			},
		},
	}, pngData)

	uc := NewVrm2PmxUsecase(Vrm2PmxUsecaseDeps{
		ModelReader: vrm.NewVrmRepository(),
		ModelWriter: pmx.NewPmxRepository(),
	})
	result, err := uc.PrepareModel(ConvertRequest{InputPath: inPath})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	outputDir := filepath.Dir(result.OutputPath)
	if _, err := os.Stat(filepath.Join(outputDir, "tex", "face.png")); err != nil {
		t.Fatalf("texture output not found: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "glTF", "sample.bin")); err != nil {
		t.Fatalf("gltf bin output not found: %v", err)
	}
}

// writeGLBForUsecaseTest はテスト用JSONをGLB形式で保存する。
func writeGLBForUsecaseTest(t *testing.T, path string, doc map[string]any, binChunk []byte) {
	t.Helper()
	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	padding := (4 - (len(jsonBytes) % 4)) % 4
	if padding > 0 {
		jsonBytes = append(jsonBytes, bytes.Repeat([]byte(" "), padding)...)
	}
	binBytes := append([]byte(nil), binChunk...)
	if len(binBytes) > 0 {
		binPadding := (4 - (len(binBytes) % 4)) % 4
		if binPadding > 0 {
			binBytes = append(binBytes, bytes.Repeat([]byte{0x00}, binPadding)...)
		}
	}

	totalLength := uint32(12 + 8 + len(jsonBytes))
	if len(binBytes) > 0 {
		totalLength += uint32(8 + len(binBytes))
	}
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
	if len(binBytes) > 0 {
		if err := binary.Write(&buf, binary.LittleEndian, uint32(len(binBytes))); err != nil {
			t.Fatalf("write bin chunk length failed: %v", err)
		}
		if err := binary.Write(&buf, binary.LittleEndian, uint32(0x004E4942)); err != nil {
			t.Fatalf("write bin chunk type failed: %v", err)
		}
		if _, err := buf.Write(binBytes); err != nil {
			t.Fatalf("write bin chunk body failed: %v", err)
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write glb file failed: %v", err)
	}
}

type prepareProgressEventCollector struct {
	events []PrepareProgressEventType
}

func (c *prepareProgressEventCollector) ReportPrepareProgress(event PrepareProgressEvent) {
	if c == nil {
		return
	}
	c.events = append(c.events, event.Type)
}

func (c *prepareProgressEventCollector) findIndex(target PrepareProgressEventType) int {
	if c == nil {
		return -1
	}
	for idx, eventType := range c.events {
		if eventType == target {
			return idx
		}
	}
	return -1
}
