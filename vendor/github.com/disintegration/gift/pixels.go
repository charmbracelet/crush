package gift

import (
	"image"
	"image/color"
	"image/draw"
)

type pixel struct {
	r, g, b, a float32
}

type imageType int

const (
	itGeneric imageType = iota
	itNRGBA
	itNRGBA64
	itRGBA
	itRGBA64
	itYCbCr
	itGray
	itGray16
	itPaletted
)

type pixelGetter struct {
	it       imageType
	bounds   image.Rectangle
	image    image.Image
	nrgba    *image.NRGBA
	nrgba64  *image.NRGBA64
	rgba     *image.RGBA
	rgba64   *image.RGBA64
	gray     *image.Gray
	gray16   *image.Gray16
	ycbcr    *image.YCbCr
	paletted *image.Paletted
	palette  []pixel
}

func newPixelGetter(img image.Image) *pixelGetter {
	switch img := img.(type) {
	case *image.NRGBA:
		return &pixelGetter{
			it:     itNRGBA,
			bounds: img.Bounds(),
			nrgba:  img,
		}

	case *image.NRGBA64:
		return &pixelGetter{
			it:      itNRGBA64,
			bounds:  img.Bounds(),
			nrgba64: img,
		}

	case *image.RGBA:
		return &pixelGetter{
			it:     itRGBA,
			bounds: img.Bounds(),
			rgba:   img,
		}

	case *image.RGBA64:
		return &pixelGetter{
			it:     itRGBA64,
			bounds: img.Bounds(),
			rgba64: img,
		}

	case *image.Gray:
		return &pixelGetter{
			it:     itGray,
			bounds: img.Bounds(),
			gray:   img,
		}

	case *image.Gray16:
		return &pixelGetter{
			it:     itGray16,
			bounds: img.Bounds(),
			gray16: img,
		}

	case *image.YCbCr:
		return &pixelGetter{
			it:     itYCbCr,
			bounds: img.Bounds(),
			ycbcr:  img,
		}

	case *image.Paletted:
		return &pixelGetter{
			it:       itPaletted,
			bounds:   img.Bounds(),
			paletted: img,
			palette:  convertPalette(img.Palette),
		}

	default:
		return &pixelGetter{
			it:     itGeneric,
			bounds: img.Bounds(),
			image:  img,
		}
	}
}

const (
	qf8  = 1.0 / 0xff
	qf16 = 1.0 / 0xffff
	epal = qf16 * qf16 / 2
)

func pixelFromColor(c color.Color) (px pixel) {
	r16, g16, b16, a16 := c.RGBA()
	switch a16 {
	case 0:
		px = pixel{0, 0, 0, 0}
	case 0xffff:
		r := float32(r16) * qf16
		g := float32(g16) * qf16
		b := float32(b16) * qf16
		px = pixel{r, g, b, 1}
	default:
		q := float32(1) / float32(a16)
		r := float32(r16) * q
		g := float32(g16) * q
		b := float32(b16) * q
		a := float32(a16) * qf16
		px = pixel{r, g, b, a}
	}
	return px
}

func convertPalette(p []color.Color) []pixel {
	pal := make([]pixel, len(p))
	for i := 0; i < len(p); i++ {
		pal[i] = pixelFromColor(p[i])
	}
	return pal
}

func getPaletteIndex(pal []pixel, px pixel) int {
	var k int
	var dmin float32 = 4
	for i, palpx := range pal {
		d := px.r - palpx.r
		dcur := d * d
		d = px.g - palpx.g
		dcur += d * d
		d = px.b - palpx.b
		dcur += d * d
		d = px.a - palpx.a
		dcur += d * d
		if dcur < epal {
			return i
		}
		if dcur < dmin {
			dmin = dcur
			k = i
		}
	}
	return k
}

func (p *pixelGetter) getPixel(x, y int) pixel {
	switch p.it {
	case itNRGBA:
		i := p.nrgba.PixOffset(x, y)
		r := float32(p.nrgba.Pix[i+0]) * qf8
		g := float32(p.nrgba.Pix[i+1]) * qf8
		b := float32(p.nrgba.Pix[i+2]) * qf8
		a := float32(p.nrgba.Pix[i+3]) * qf8
		return pixel{r, g, b, a}

	case itNRGBA64:
		i := p.nrgba64.PixOffset(x, y)
		r := float32(uint16(p.nrgba64.Pix[i+0])<<8|uint16(p.nrgba64.Pix[i+1])) * qf16
		g := float32(uint16(p.nrgba64.Pix[i+2])<<8|uint16(p.nrgba64.Pix[i+3])) * qf16
		b := float32(uint16(p.nrgba64.Pix[i+4])<<8|uint16(p.nrgba64.Pix[i+5])) * qf16
		a := float32(uint16(p.nrgba64.Pix[i+6])<<8|uint16(p.nrgba64.Pix[i+7])) * qf16
		return pixel{r, g, b, a}

	case itRGBA:
		i := p.rgba.PixOffset(x, y)
		a8 := p.rgba.Pix[i+3]
		switch a8 {
		case 0xff:
			r := float32(p.rgba.Pix[i+0]) * qf8
			g := float32(p.rgba.Pix[i+1]) * qf8
			b := float32(p.rgba.Pix[i+2]) * qf8
			return pixel{r, g, b, 1}
		case 0:
			return pixel{0, 0, 0, 0}
		default:
			q := float32(1) / float32(a8)
			r := float32(p.rgba.Pix[i+0]) * q
			g := float32(p.rgba.Pix[i+1]) * q
			b := float32(p.rgba.Pix[i+2]) * q
			a := float32(a8) * qf8
			return pixel{r, g, b, a}
		}

	case itRGBA64:
		i := p.rgba64.PixOffset(x, y)
		a16 := uint16(p.rgba64.Pix[i+6])<<8 | uint16(p.rgba64.Pix[i+7])
		switch a16 {
		case 0xffff:
			r := float32(uint16(p.rgba64.Pix[i+0])<<8|uint16(p.rgba64.Pix[i+1])) * qf16
			g := float32(uint16(p.rgba64.Pix[i+2])<<8|uint16(p.rgba64.Pix[i+3])) * qf16
			b := float32(uint16(p.rgba64.Pix[i+4])<<8|uint16(p.rgba64.Pix[i+5])) * qf16
			return pixel{r, g, b, 1}
		case 0:
			return pixel{0, 0, 0, 0}
		default:
			q := float32(1) / float32(a16)
			r := float32(uint16(p.rgba64.Pix[i+0])<<8|uint16(p.rgba64.Pix[i+1])) * q
			g := float32(uint16(p.rgba64.Pix[i+2])<<8|uint16(p.rgba64.Pix[i+3])) * q
			b := float32(uint16(p.rgba64.Pix[i+4])<<8|uint16(p.rgba64.Pix[i+5])) * q
			a := float32(a16) * qf16
			return pixel{r, g, b, a}
		}

	case itGray:
		i := p.gray.PixOffset(x, y)
		v := float32(p.gray.Pix[i]) * qf8
		return pixel{v, v, v, 1}

	case itGray16:
		i := p.gray16.PixOffset(x, y)
		v := float32(uint16(p.gray16.Pix[i+0])<<8|uint16(p.gray16.Pix[i+1])) * qf16
		return pixel{v, v, v, 1}

	case itYCbCr:
		iy := (y-p.ycbcr.Rect.Min.Y)*p.ycbcr.YStride + (x - p.ycbcr.Rect.Min.X)

		var ic int
		switch p.ycbcr.SubsampleRatio {
		case image.YCbCrSubsampleRatio444:
			ic = (y-p.ycbcr.Rect.Min.Y)*p.ycbcr.CStride + (x - p.ycbcr.Rect.Min.X)
		case image.YCbCrSubsampleRatio422:
			ic = (y-p.ycbcr.Rect.Min.Y)*p.ycbcr.CStride + (x/2 - p.ycbcr.Rect.Min.X/2)
		case image.YCbCrSubsampleRatio420:
			ic = (y/2-p.ycbcr.Rect.Min.Y/2)*p.ycbcr.CStride + (x/2 - p.ycbcr.Rect.Min.X/2)
		case image.YCbCrSubsampleRatio440:
			ic = (y/2-p.ycbcr.Rect.Min.Y/2)*p.ycbcr.CStride + (x - p.ycbcr.Rect.Min.X)
		default:
			ic = p.ycbcr.COffset(x, y)
		}

		const (
			max = 255 * 1e5
			inv = 1.0 / max
		)

		y1 := int32(p.ycbcr.Y[iy]) * 1e5
		cb1 := int32(p.ycbcr.Cb[ic]) - 128
		cr1 := int32(p.ycbcr.Cr[ic]) - 128

		r1 := y1 + 140200*cr1
		g1 := y1 - 34414*cb1 - 71414*cr1
		b1 := y1 + 177200*cb1

		r := float32(clampi32(r1, 0, max)) * inv
		g := float32(clampi32(g1, 0, max)) * inv
		b := float32(clampi32(b1, 0, max)) * inv

		return pixel{r, g, b, 1}

	case itPaletted:
		i := p.paletted.PixOffset(x, y)
		k := p.paletted.Pix[i]
		return p.palette[k]
	}

	return pixelFromColor(p.image.At(x, y))
}

func (p *pixelGetter) getPixelRow(y int, buf *[]pixel) {
	*buf = (*buf)[:0]
	for x := p.bounds.Min.X; x != p.bounds.Max.X; x++ {
		*buf = append(*buf, p.getPixel(x, y))
	}
}

func (p *pixelGetter) getPixelColumn(x int, buf *[]pixel) {
	*buf = (*buf)[:0]
	for y := p.bounds.Min.Y; y != p.bounds.Max.Y; y++ {
		*buf = append(*buf, p.getPixel(x, y))
	}
}

func f32u8(val float32) uint8 {
	x := int64(val + 0.5)
	if x > 0xff {
		return 0xff
	}
	if x > 0 {
		return uint8(x)
	}
	return 0
}

func f32u16(val float32) uint16 {
	x := int64(val + 0.5)
	if x > 0xffff {
		return 0xffff
	}
	if x > 0 {
		return uint16(x)
	}
	return 0
}

func clampi32(val, min, max int32) int32 {
	if val > max {
		return max
	}
	if val > min {
		return val
	}
	return 0
}

type pixelSetter struct {
	it       imageType
	bounds   image.Rectangle
	image    draw.Image
	nrgba    *image.NRGBA
	nrgba64  *image.NRGBA64
	rgba     *image.RGBA
	rgba64   *image.RGBA64
	gray     *image.Gray
	gray16   *image.Gray16
	paletted *image.Paletted
	palette  []pixel
}

func newPixelSetter(img draw.Image) *pixelSetter {
	switch img := img.(type) {
	case *image.NRGBA:
		return &pixelSetter{
			it:     itNRGBA,
			bounds: img.Bounds(),
			nrgba:  img,
		}

	case *image.NRGBA64:
		return &pixelSetter{
			it:      itNRGBA64,
			bounds:  img.Bounds(),
			nrgba64: img,
		}

	case *image.RGBA:
		return &pixelSetter{
			it:     itRGBA,
			bounds: img.Bounds(),
			rgba:   img,
		}

	case *image.RGBA64:
		return &pixelSetter{
			it:     itRGBA64,
			bounds: img.Bounds(),
			rgba64: img,
		}

	case *image.Gray:
		return &pixelSetter{
			it:     itGray,
			bounds: img.Bounds(),
			gray:   img,
		}

	case *image.Gray16:
		return &pixelSetter{
			it:     itGray16,
			bounds: img.Bounds(),
			gray16: img,
		}

	case *image.Paletted:
		return &pixelSetter{
			it:       itPaletted,
			bounds:   img.Bounds(),
			paletted: img,
			palette:  convertPalette(img.Palette),
		}

	default:
		return &pixelSetter{
			it:     itGeneric,
			bounds: img.Bounds(),
			image:  img,
		}
	}
}

func (p *pixelSetter) setPixel(x, y int, px pixel) {
	if !image.Pt(x, y).In(p.bounds) {
		return
	}
	switch p.it {
	case itNRGBA:
		i := p.nrgba.PixOffset(x, y)
		p.nrgba.Pix[i+0] = f32u8(px.r * 0xff)
		p.nrgba.Pix[i+1] = f32u8(px.g * 0xff)
		p.nrgba.Pix[i+2] = f32u8(px.b * 0xff)
		p.nrgba.Pix[i+3] = f32u8(px.a * 0xff)

	case itNRGBA64:
		r16 := f32u16(px.r * 0xffff)
		g16 := f32u16(px.g * 0xffff)
		b16 := f32u16(px.b * 0xffff)
		a16 := f32u16(px.a * 0xffff)
		i := p.nrgba64.PixOffset(x, y)
		p.nrgba64.Pix[i+0] = uint8(r16 >> 8)
		p.nrgba64.Pix[i+1] = uint8(r16 & 0xff)
		p.nrgba64.Pix[i+2] = uint8(g16 >> 8)
		p.nrgba64.Pix[i+3] = uint8(g16 & 0xff)
		p.nrgba64.Pix[i+4] = uint8(b16 >> 8)
		p.nrgba64.Pix[i+5] = uint8(b16 & 0xff)
		p.nrgba64.Pix[i+6] = uint8(a16 >> 8)
		p.nrgba64.Pix[i+7] = uint8(a16 & 0xff)

	case itRGBA:
		fa := px.a * 0xff
		i := p.rgba.PixOffset(x, y)
		p.rgba.Pix[i+0] = f32u8(px.r * fa)
		p.rgba.Pix[i+1] = f32u8(px.g * fa)
		p.rgba.Pix[i+2] = f32u8(px.b * fa)
		p.rgba.Pix[i+3] = f32u8(fa)

	case itRGBA64:
		fa := px.a * 0xffff
		r16 := f32u16(px.r * fa)
		g16 := f32u16(px.g * fa)
		b16 := f32u16(px.b * fa)
		a16 := f32u16(fa)
		i := p.rgba64.PixOffset(x, y)
		p.rgba64.Pix[i+0] = uint8(r16 >> 8)
		p.rgba64.Pix[i+1] = uint8(r16 & 0xff)
		p.rgba64.Pix[i+2] = uint8(g16 >> 8)
		p.rgba64.Pix[i+3] = uint8(g16 & 0xff)
		p.rgba64.Pix[i+4] = uint8(b16 >> 8)
		p.rgba64.Pix[i+5] = uint8(b16 & 0xff)
		p.rgba64.Pix[i+6] = uint8(a16 >> 8)
		p.rgba64.Pix[i+7] = uint8(a16 & 0xff)

	case itGray:
		i := p.gray.PixOffset(x, y)
		p.gray.Pix[i] = f32u8((0.299*px.r + 0.587*px.g + 0.114*px.b) * px.a * 0xff)

	case itGray16:
		i := p.gray16.PixOffset(x, y)
		y16 := f32u16((0.299*px.r + 0.587*px.g + 0.114*px.b) * px.a * 0xffff)
		p.gray16.Pix[i+0] = uint8(y16 >> 8)
		p.gray16.Pix[i+1] = uint8(y16 & 0xff)

	case itPaletted:
		px1 := pixel{
			minf32(maxf32(px.r, 0), 1),
			minf32(maxf32(px.g, 0), 1),
			minf32(maxf32(px.b, 0), 1),
			minf32(maxf32(px.a, 0), 1),
		}
		i := p.paletted.PixOffset(x, y)
		k := getPaletteIndex(p.palette, px1)
		p.paletted.Pix[i] = uint8(k)

	case itGeneric:
		r16 := f32u16(px.r * 0xffff)
		g16 := f32u16(px.g * 0xffff)
		b16 := f32u16(px.b * 0xffff)
		a16 := f32u16(px.a * 0xffff)
		p.image.Set(x, y, color.NRGBA64{r16, g16, b16, a16})
	}
}

func (p *pixelSetter) setPixelRow(y int, buf []pixel) {
	for i, x := 0, p.bounds.Min.X; i < len(buf); i, x = i+1, x+1 {
		p.setPixel(x, y, buf[i])
	}
}

func (p *pixelSetter) setPixelColumn(x int, buf []pixel) {
	for i, y := 0, p.bounds.Min.Y; i < len(buf); i, y = i+1, y+1 {
		p.setPixel(x, y, buf[i])
	}
}
