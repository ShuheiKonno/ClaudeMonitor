package main

import (
	"image/png"
	"os"
	"testing"
)

// 各サイズの PNG を assets/preview/ に吐き出して目視確認用とする。
// go test -run TestDumpSizes ./cmd/genicon で実行。
func TestDumpSizes(t *testing.T) {
	if os.Getenv("DUMP_SIZES") == "" {
		t.Skip("DUMP_SIZES=1 で有効化")
	}
	for _, s := range []int{16, 20, 24, 32, 48, 64, 128} {
		img := drawAppIcon(s)
		f, err := os.Create("../../assets/preview-" + itoa(s) + ".png")
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
