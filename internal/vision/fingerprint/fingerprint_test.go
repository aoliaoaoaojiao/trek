package fingerprint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHammingDistance_SameFingerprint(t *testing.T) {
	a := "IMGPage:04001d00000800007d7f7f7f7f7f7f7f005a520008101040005a520000000040"
	d := HammingDistance(a, a)
	if d != 0 {
		t.Fatalf("同一指纹的 Hamming 距离应为 0，实际: %d", d)
	}
}

func TestHammingDistance_DifferentLengths(t *testing.T) {
	a := "IMGPage:04001d00000800007d7f7f7f7f7f7f7f005a520008101040005a520000000040"
	b := "IMGPage:04001d00000800007d7f7f7f7f7f7f7f00"
	d := HammingDistance(a, b)
	if d != -1 {
		t.Fatalf("不同长度指纹应返回 -1，实际: %d", d)
	}
}

func TestHammingDistance_InvalidHex(t *testing.T) {
	d := HammingDistance("IMGPage:zzzz", "IMGPage:0000")
	if d != -1 {
		t.Fatalf("非法 hex 应返回 -1，实际: %d", d)
	}
}

func TestHammingDistance_StarScreenshots(t *testing.T) {
	starDir := filepath.Join("..", "..", "..", "testdata", "ImgPageTest", "star")
	images := []string{"star_a.png", "star_b.png", "star_c.png"}

	hashes := make([]string, len(images))
	for i, name := range images {
		data, err := os.ReadFile(filepath.Join(starDir, name))
		if err != nil {
			t.Fatalf("读取测试图片失败: %v", err)
		}
		hash := Name(data, nil)
		if hash == "" {
			t.Fatalf("图片 %s 指纹为空", name)
		}
		hashes[i] = hash
		t.Logf("图片 %s 指纹: %s", name, hash)
	}

	// 三张图的指纹应互不相同
	if hashes[0] == hashes[1] {
		t.Fatalf("star_a 和 star_b 指纹不应相同: %s", hashes[0])
	}
	if hashes[0] == hashes[2] {
		t.Fatalf("star_a 和 star_c 指纹不应相同: %s", hashes[0])
	}
	if hashes[1] == hashes[2] {
		t.Fatalf("star_b 和 star_c 指纹不应相同: %s", hashes[1])
	}

	// 但 Hamming 距离应较小（同一界面的微小差异）
	for i := 0; i < len(hashes); i++ {
		for j := i + 1; j < len(hashes); j++ {
			d := HammingDistance(hashes[i], hashes[j])
			if d < 0 {
				t.Fatalf("HammingDistance(%d, %d) 返回无效值: %d", i, j, d)
			}
			t.Logf("star_%c 与 star_%c 的 Hamming 距离: %d", 'a'+i, 'a'+j, d)
			if d > 20 {
				t.Fatalf("同一界面的 Hamming 距离应较小（<20），实际: %d", d)
			}
		}
	}
}
