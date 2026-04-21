// genicon は Claude Monitor のアイコンアセット（assets/icon.ico, assets/icon-preview.png）を
// 生成するためのスタンドアローンツール。
//
//	go run ./cmd/genicon
//
// デザインは「円環ゲージ + 中央シンボル」をコンセプトとし、アプリアイコン（静的）は
// Claude 系オレンジのフルリング + 中央の "C"、トレイアイコン（動的）は 7 日使用率に
// 応じたリングフィル + 中央の 5 時間使用率数値で描画する（トレイ側は tray.go が担当）。
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// カラーパレット（Claude のオレンジを基調とした暖色ダーク）
var (
	colorBgDisk    = color.RGBA{R: 30, G: 27, B: 35, A: 255}   // #1E1B23 中央ダークディスク
	colorRingTrack = color.RGBA{R: 58, G: 46, B: 54, A: 255}   // #3A2E36 リング溝
	colorAccent    = color.RGBA{R: 217, G: 119, B: 87, A: 255} // #D97757 Claude ブランドオレンジ
	colorCream     = color.RGBA{R: 245, G: 240, B: 234, A: 255} // #F5F0EA 明色
	colorAccentHi  = color.RGBA{R: 234, G: 142, B: 106, A: 255} // #EA8E6A ハイライト
)

func main() {
	sizes := []int{16, 20, 24, 32, 40, 48, 64, 128, 256}
	var imgs []image.Image
	for _, s := range sizes {
		imgs = append(imgs, drawAppIcon(s))
	}

	if err := os.MkdirAll("assets", 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	icoPath := filepath.Join("assets", "icon.ico")
	if err := writeICO(icoPath, imgs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote", icoPath)

	previewPath := filepath.Join("assets", "icon-preview.png")
	prev, err := os.Create(previewPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := png.Encode(prev, drawAppIcon(512)); err != nil {
		prev.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	prev.Close()
	fmt.Println("wrote", previewPath)
}

// drawAppIcon はアプリアイコンを 1 枚描画する。
// 外側に厚めのオレンジリング、内側にダークディスク、中央に幾何学的な "C" を描画する。
func drawAppIcon(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	cx := float64(size) / 2
	cy := float64(size) / 2
	// リングの外径 / 内径（サイズ比）
	outerR := float64(size) * 0.48
	innerR := float64(size) * 0.36

	// ダーク中央円 → リング溝 → リング本体 の順で重ね描き
	fillDisk(img, cx, cy, outerR, colorRingTrack)
	// リング全周をオレンジで塗る（静的アイコンは 100%）
	fillRing(img, cx, cy, innerR, outerR, -math.Pi/2, 2*math.Pi, colorAccent)
	// 12 時位置に明るいアクセントマーカー（ブランド感）
	markerSweep := math.Pi / 14
	fillRing(img, cx, cy, innerR, outerR, -math.Pi/2-markerSweep/2, markerSweep, colorAccentHi)
	// 中央ダークディスク
	fillDisk(img, cx, cy, innerR-float64(size)*0.01, colorBgDisk)
	// 中央 "C"
	drawLetterC(img, cx, cy, float64(size), colorCream)

	return img
}

// fillDisk: 円盤を塗る（アンチエイリアス付き）。
func fillDisk(img *image.RGBA, cx, cy, r float64, c color.RGBA) {
	b := img.Bounds()
	minX := int(math.Floor(cx - r - 1))
	maxX := int(math.Ceil(cx + r + 1))
	minY := int(math.Floor(cy - r - 1))
	maxY := int(math.Ceil(cy + r + 1))
	if minX < b.Min.X {
		minX = b.Min.X
	}
	if maxX > b.Max.X {
		maxX = b.Max.X
	}
	if minY < b.Min.Y {
		minY = b.Min.Y
	}
	if maxY > b.Max.Y {
		maxY = b.Max.Y
	}
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			d := math.Sqrt(dx*dx + dy*dy)
			if d <= r-0.5 {
				blendOver(img, x, y, c, 1)
				continue
			}
			if d < r+0.5 {
				blendOver(img, x, y, c, r+0.5-d)
			}
		}
	}
}

// fillRing: 円環（ドーナツ状）を指定角度範囲で塗る。
// startAngle: ラジアン、-pi/2 が 12 時方向。sweep: 時計回り正。
func fillRing(img *image.RGBA, cx, cy, innerR, outerR, startAngle, sweep float64, c color.RGBA) {
	if sweep <= 0 {
		return
	}
	b := img.Bounds()
	minX := int(math.Floor(cx - outerR - 1))
	maxX := int(math.Ceil(cx + outerR + 1))
	minY := int(math.Floor(cy - outerR - 1))
	maxY := int(math.Ceil(cy + outerR + 1))
	if minX < b.Min.X {
		minX = b.Min.X
	}
	if maxX > b.Max.X {
		maxX = b.Max.X
	}
	if minY < b.Min.Y {
		minY = b.Min.Y
	}
	if maxY > b.Max.Y {
		maxY = b.Max.Y
	}
	// 角度 a (atan2) を [startAngle, startAngle+2π) に正規化して sweep 範囲判定。
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			d := math.Sqrt(dx*dx + dy*dy)
			if d > outerR+0.5 || d < innerR-0.5 {
				continue
			}
			a := math.Atan2(dy, dx)
			// 正規化
			for a < startAngle {
				a += 2 * math.Pi
			}
			for a >= startAngle+2*math.Pi {
				a -= 2 * math.Pi
			}
			if a-startAngle > sweep {
				continue
			}
			alpha := 1.0
			if d > outerR-0.5 {
				alpha = math.Min(alpha, outerR+0.5-d)
			}
			if d < innerR+0.5 {
				alpha = math.Min(alpha, d-(innerR-0.5))
			}
			if alpha <= 0 {
				continue
			}
			blendOver(img, x, y, c, alpha)
		}
	}
}

// drawLetterC: 幾何学的な "C" を中央に描く（アーク + 両端丸キャップ）。
// size は画像の一辺サイズ。
func drawLetterC(img *image.RGBA, cx, cy, size float64, c color.RGBA) {
	// 小さいサイズでは単純な「C」文字形状が潰れるので、
	// 16px 以下はディスクを一段暗くした記号のみに切り替える。
	if size <= 24 {
		// 小サイズは中央の "C" を省略し、ディスクを一段明るくして識別感を出す
		// ※ 実際はトレイ側は tray.go が数値を描画するのでここは主にアプリアイコンの
		//    最小サイズ（16/20/24）のフォールバック。
		return
	}
	// "C" は開口部を右にした円弧。外径 outerR・内径 innerR、開口角度 gap。
	rOuter := size * 0.30
	rInner := size * 0.20
	gap := 80.0 * math.Pi / 180.0 // 80 度の開口
	// atan2 座標で +x(右)=0、+y(下)=π/2。右側に gap を空けたいので、
	// 開始角を gap/2（右下寄り）から時計回りに 360-80 度スイープ。
	start := gap / 2
	sweep := 2*math.Pi - gap
	// 本体アーク
	fillArc(img, cx, cy, rInner, rOuter, start, sweep, c)
	// 両端の丸キャップ
	midR := (rInner + rOuter) / 2
	capR := (rOuter - rInner) / 2
	endAngle := start + sweep
	cx1 := cx + midR*math.Cos(start)
	cy1 := cy + midR*math.Sin(start)
	cx2 := cx + midR*math.Cos(endAngle)
	cy2 := cy + midR*math.Sin(endAngle)
	fillDisk(img, cx1, cy1, capR, c)
	fillDisk(img, cx2, cy2, capR, c)
}

// fillArc: アーク（startAngle から sweep ラジアン、時計回り正）を描画。fillRing と同じ。
func fillArc(img *image.RGBA, cx, cy, innerR, outerR, startAngle, sweep float64, c color.RGBA) {
	fillRing(img, cx, cy, innerR, outerR, startAngle, sweep, c)
}

// blendOver: premultiplied alpha 合成で img[x,y] に色 c を alpha でソースオーバーする。
func blendOver(img *image.RGBA, x, y int, c color.RGBA, alpha float64) {
	if alpha <= 0 {
		return
	}
	if alpha > 1 {
		alpha = 1
	}
	i := img.PixOffset(x, y)
	sa := float64(c.A) / 255 * alpha
	sr := float64(c.R) / 255 * sa
	sg := float64(c.G) / 255 * sa
	sb := float64(c.B) / 255 * sa
	// 既存ピクセルは straight alpha
	dr := float64(img.Pix[i+0]) / 255
	dg := float64(img.Pix[i+1]) / 255
	db := float64(img.Pix[i+2]) / 255
	da := float64(img.Pix[i+3]) / 255
	outA := sa + da*(1-sa)
	var outR, outG, outB float64
	if outA > 0 {
		outR = (sr + dr*da*(1-sa)) / outA
		outG = (sg + dg*da*(1-sa)) / outA
		outB = (sb + db*da*(1-sa)) / outA
	}
	img.Pix[i+0] = clamp8(outR * 255)
	img.Pix[i+1] = clamp8(outG * 255)
	img.Pix[i+2] = clamp8(outB * 255)
	img.Pix[i+3] = clamp8(outA * 255)
}

func clamp8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(math.Round(v))
}

// writeICO: PNG 埋め込み形式のマルチサイズ .ico を書き出す。Windows Vista 以降対応。
func writeICO(path string, imgs []image.Image) error {
	type entry struct {
		size int
		data []byte
		off  uint32
	}
	entries := make([]entry, 0, len(imgs))
	for _, im := range imgs {
		var buf bytes.Buffer
		if err := png.Encode(&buf, im); err != nil {
			return err
		}
		entries = append(entries, entry{size: im.Bounds().Dx(), data: buf.Bytes()})
	}
	dirSize := 6 + 16*len(entries)
	off := uint32(dirSize)
	for i := range entries {
		entries[i].off = off
		off += uint32(len(entries[i].data))
	}
	var out bytes.Buffer
	// ICO header
	binary.Write(&out, binary.LittleEndian, uint16(0))              // reserved
	binary.Write(&out, binary.LittleEndian, uint16(1))              // type=ICO
	binary.Write(&out, binary.LittleEndian, uint16(len(entries))) // count
	for _, e := range entries {
		w := byte(e.size)
		h := byte(e.size)
		if e.size >= 256 {
			w = 0
			h = 0
		}
		out.WriteByte(w)
		out.WriteByte(h)
		out.WriteByte(0)                                          // color count (0 = >=256)
		out.WriteByte(0)                                          // reserved
		binary.Write(&out, binary.LittleEndian, uint16(1))        // planes
		binary.Write(&out, binary.LittleEndian, uint16(32))       // bpp
		binary.Write(&out, binary.LittleEndian, uint32(len(e.data)))
		binary.Write(&out, binary.LittleEndian, e.off)
	}
	for _, e := range entries {
		out.Write(e.data)
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}
