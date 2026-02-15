// 指示: miu200521358
package vrm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportEmbeddedSpecialEyeTextures(t *testing.T) {
	tempDir := t.TempDir()
	written, err := ExportEmbeddedSpecialEyeTextures(tempDir)
	if err != nil {
		t.Fatalf("export embedded special eye textures failed: %v", err)
	}
	if len(written) != len(specialEyeEmbeddedTextureAssetFileNames) {
		t.Fatalf("written file count mismatch: got=%d want=%d", len(written), len(specialEyeEmbeddedTextureAssetFileNames))
	}
	for _, fileName := range specialEyeEmbeddedTextureAssetFileNames {
		if _, err := os.Stat(filepath.Join(tempDir, fileName)); err != nil {
			t.Fatalf("embedded texture not found: file=%s err=%v", fileName, err)
		}
	}
}
