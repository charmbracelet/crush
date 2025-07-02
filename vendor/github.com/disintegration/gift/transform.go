package gift

import (
	"image"
	"image/color"
	"image/draw"
)

type transformType int

const (
	ttRotate90 transformType = iota
	ttRotate180
	ttRotate270
	ttFlipHorizontal
	ttFlipVertical
	ttTranspose
	ttTransverse
)

type transformFilter struct {
	tt transformType
}

func (p *transformFilter) Bounds(srcBounds image.Rectangle) (dstBounds image.Rectangle) {
	if p.tt == ttRotate90 || p.tt == ttRotate270 || p.tt == ttTranspose || p.tt == ttTransverse {
		dstBounds = image.Rect(0, 0, srcBounds.Dy(), srcBounds.Dx())
	} else {
		dstBounds = image.Rect(0, 0, srcBounds.Dx(), srcBounds.Dy())
	}
	return
}

func (p *transformFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	if options == nil {
		options = &defaultOptions
	}

	srcb := src.Bounds()
	dstb := dst.Bounds()

	pixGetter := newPixelGetter(src)
	pixSetter := newPixelSetter(dst)

	parallelize(options.Parallelization, srcb.Min.Y, srcb.Max.Y, func(pmin, pmax int) {
		for srcy := pmin; srcy < pmax; srcy++ {
			for srcx := srcb.Min.X; srcx < srcb.Max.X; srcx++ {
				var dstx, dsty int
				switch p.tt {
				case ttRotate90:
					dstx = dstb.Min.X + srcy - srcb.Min.Y
					dsty = dstb.Min.Y + srcb.Max.X - srcx - 1
				case ttRotate180:
					dstx = dstb.Min.X + srcb.Max.X - srcx - 1
					dsty = dstb.Min.Y + srcb.Max.Y - srcy - 1
				case ttRotate270:
					dstx = dstb.Min.X + srcb.Max.Y - srcy - 1
					dsty = dstb.Min.Y + srcx - srcb.Min.X
				case ttFlipHorizontal:
					dstx = dstb.Min.X + srcb.Max.X - srcx - 1
					dsty = dstb.Min.Y + srcy - srcb.Min.Y
				case ttFlipVertical:
					dstx = dstb.Min.X + srcx - srcb.Min.X
					dsty = dstb.Min.Y + srcb.Max.Y - srcy - 1
				case ttTranspose:
					dstx = dstb.Min.X + srcy - srcb.Min.Y
					dsty = dstb.Min.Y + srcx - srcb.Min.X
				case ttTransverse:
					dstx = dstb.Min.Y + srcb.Max.Y - srcy - 1
					dsty = dstb.Min.X + srcb.Max.X - srcx - 1
				}
				pixSetter.setPixel(dstx, dsty, pixGetter.getPixel(srcx, srcy))
			}
		}
	})
}

// Rotate90 creates a filter that rotates an image 90 degrees counter-clockwise.
func Rotate90() Filter {
	return &transformFilter{
		tt: ttRotate90,
	}
}

// Rotate180 creates a filter that rotates an image 180 degrees counter-clockwise.
func Rotate180() Filter {
	return &transformFilter{
		tt: ttRotate180,
	}
}

// Rotate270 creates a filter that rotates an image 270 degrees counter-clockwise.
func Rotate270() Filter {
	return &transformFilter{
		tt: ttRotate270,
	}
}

// FlipHorizontal creates a filter that flips an image horizontally.
func FlipHorizontal() Filter {
	return &transformFilter{
		tt: ttFlipHorizontal,
	}
}

// FlipVertical creates a filter that flips an image vertically.
func FlipVertical() Filter {
	return &transformFilter{
		tt: ttFlipVertical,
	}
}

// Transpose creates a filter that flips an image horizontally and rotates 90 degrees counter-clockwise.
func Transpose() Filter {
	return &transformFilter{
		tt: ttTranspose,
	}
}

// Transverse creates a filter that flips an image vertically and rotates 90 degrees counter-clockwise.
func Transverse() Filter {
	return &transformFilter{
		tt: ttTransverse,
	}
}

// Interpolation is an interpolation algorithm used for image transformation.
type Interpolation int

const (
	// NearestNeighborInterpolation is a nearest-neighbor interpolation algorithm.
	NearestNeighborInterpolation Interpolation = iota
	// LinearInterpolation is a bilinear interpolation algorithm.
	LinearInterpolation
	// CubicInterpolation is a bicubic interpolation algorithm.
	CubicInterpolation
)

func rotatePoint(x, y, asin, acos float32) (float32, float32) {
	newx := x*acos - y*asin
	newy := x*asin + y*acos
	return newx, newy
}

func calcRotatedSize(w, h int, angle float32) (int, int) {
	if w <= 0 || h <= 0 {
		return 0, 0
	}

	xoff := float32(w)/2 - 0.5
	yoff := float32(h)/2 - 0.5

	asin, acos := sincosf32(angle)
	x1, y1 := rotatePoint(0-xoff, 0-yoff, asin, acos)
	x2, y2 := rotatePoint(float32(w-1)-xoff, 0-yoff, asin, acos)
	x3, y3 := rotatePoint(float32(w-1)-xoff, float32(h-1)-yoff, asin, acos)
	x4, y4 := rotatePoint(0-xoff, float32(h-1)-yoff, asin, acos)

	minx := minf32(x1, minf32(x2, minf32(x3, x4)))
	maxx := maxf32(x1, maxf32(x2, maxf32(x3, x4)))
	miny := minf32(y1, minf32(y2, minf32(y3, y4)))
	maxy := maxf32(y1, maxf32(y2, maxf32(y3, y4)))

	neww := maxx - minx + 1
	if neww-floorf32(neww) > 0.01 {
		neww += 2
	}
	newh := maxy - miny + 1
	if newh-floorf32(newh) > 0.01 {
		newh += 2
	}
	return int(neww), int(newh)
}

type rotateFilter struct {
	angle         float32
	bgcolor       color.Color
	interpolation Interpolation
}

func (p *rotateFilter) Bounds(srcBounds image.Rectangle) (dstBounds image.Rectangle) {
	w, h := calcRotatedSize(srcBounds.Dx(), srcBounds.Dy(), p.angle)
	dstBounds = image.Rect(0, 0, w, h)
	return
}

func (p *rotateFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	if options == nil {
		options = &defaultOptions
	}

	srcb := src.Bounds()
	dstb := dst.Bounds()

	w, h := calcRotatedSize(srcb.Dx(), srcb.Dy(), p.angle)
	if w <= 0 || h <= 0 {
		return
	}

	srcxoff := float32(srcb.Dx())/2 - 0.5
	srcyoff := float32(srcb.Dy())/2 - 0.5
	dstxoff := float32(w)/2 - 0.5
	dstyoff := float32(h)/2 - 0.5

	bgpx := pixelFromColor(p.bgcolor)
	asin, acos := sincosf32(p.angle)

	pixGetter := newPixelGetter(src)
	pixSetter := newPixelSetter(dst)

	parallelize(options.Parallelization, 0, h, func(pmin, pmax int) {
		for y := pmin; y < pmax; y++ {
			for x := 0; x < w; x++ {

				xf, yf := rotatePoint(float32(x)-dstxoff, float32(y)-dstyoff, asin, acos)
				xf, yf = float32(srcb.Min.X)+xf+srcxoff, float32(srcb.Min.Y)+yf+srcyoff
				var px pixel

				switch p.interpolation {
				case CubicInterpolation:
					px = interpolateCubic(xf, yf, srcb, pixGetter, bgpx)
				case LinearInterpolation:
					px = interpolateLinear(xf, yf, srcb, pixGetter, bgpx)
				default:
					px = interpolateNearest(xf, yf, srcb, pixGetter, bgpx)
				}

				pixSetter.setPixel(dstb.Min.X+x, dstb.Min.Y+y, px)
			}
		}
	})
}

func interpolateCubic(xf, yf float32, bounds image.Rectangle, pixGetter *pixelGetter, bgpx pixel) pixel {
	var pxs [16]pixel
	var cfs [16]float32
	var px pixel

	x0, y0 := int(floorf32(xf)), int(floorf32(yf))
	if !image.Pt(x0, y0).In(image.Rect(bounds.Min.X-1, bounds.Min.Y-1, bounds.Max.X, bounds.Max.Y)) {
		return bgpx
	}
	xq, yq := xf-float32(x0), yf-float32(y0)

	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			pt := image.Pt(x0+j-1, y0+i-1)
			if pt.In(bounds) {
				pxs[i*4+j] = pixGetter.getPixel(pt.X, pt.Y)
			} else {
				pxs[i*4+j] = bgpx
			}
		}
	}

	const (
		k04 = 1 / 4.0
		k12 = 1 / 12.0
		k36 = 1 / 36.0
	)

	cfs[0] = k36 * xq * yq * (xq - 1) * (xq - 2) * (yq - 1) * (yq - 2)
	cfs[1] = -k12 * yq * (xq - 1) * (xq - 2) * (xq + 1) * (yq - 1) * (yq - 2)
	cfs[2] = k12 * xq * yq * (xq + 1) * (xq - 2) * (yq - 1) * (yq - 2)
	cfs[3] = -k36 * xq * yq * (xq - 1) * (xq + 1) * (yq - 1) * (yq - 2)
	cfs[4] = -k12 * xq * (xq - 1) * (xq - 2) * (yq - 1) * (yq - 2) * (yq + 1)
	cfs[5] = k04 * (xq - 1) * (xq - 2) * (xq + 1) * (yq - 1) * (yq - 2) * (yq + 1)
	cfs[6] = -k04 * xq * (xq + 1) * (xq - 2) * (yq - 1) * (yq - 2) * (yq + 1)
	cfs[7] = k12 * xq * (xq - 1) * (xq + 1) * (yq - 1) * (yq - 2) * (yq + 1)
	cfs[8] = k12 * xq * yq * (xq - 1) * (xq - 2) * (yq + 1) * (yq - 2)
	cfs[9] = -k04 * yq * (xq - 1) * (xq - 2) * (xq + 1) * (yq + 1) * (yq - 2)
	cfs[10] = k04 * xq * yq * (xq + 1) * (xq - 2) * (yq + 1) * (yq - 2)
	cfs[11] = -k12 * xq * yq * (xq - 1) * (xq + 1) * (yq + 1) * (yq - 2)
	cfs[12] = -k36 * xq * yq * (xq - 1) * (xq - 2) * (yq - 1) * (yq + 1)
	cfs[13] = k12 * yq * (xq - 1) * (xq - 2) * (xq + 1) * (yq - 1) * (yq + 1)
	cfs[14] = -k12 * xq * yq * (xq + 1) * (xq - 2) * (yq - 1) * (yq + 1)
	cfs[15] = k36 * xq * yq * (xq - 1) * (xq + 1) * (yq - 1) * (yq + 1)

	for i := range pxs {
		wa := pxs[i].a * cfs[i]
		px.r += pxs[i].r * wa
		px.g += pxs[i].g * wa
		px.b += pxs[i].b * wa
		px.a += wa
	}

	if px.a != 0 {
		px.r /= px.a
		px.g /= px.a
		px.b /= px.a
	}

	return px
}

func interpolateLinear(xf, yf float32, bounds image.Rectangle, pixGetter *pixelGetter, bgpx pixel) pixel {
	var pxs [4]pixel
	var cfs [4]float32
	var px pixel

	x0, y0 := int(floorf32(xf)), int(floorf32(yf))
	if !image.Pt(x0, y0).In(image.Rect(bounds.Min.X-1, bounds.Min.Y-1, bounds.Max.X, bounds.Max.Y)) {
		return bgpx
	}
	xq, yq := xf-float32(x0), yf-float32(y0)

	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			pt := image.Pt(x0+j, y0+i)
			if pt.In(bounds) {
				pxs[i*2+j] = pixGetter.getPixel(pt.X, pt.Y)
			} else {
				pxs[i*2+j] = bgpx
			}
		}
	}

	cfs[0] = (1 - xq) * (1 - yq)
	cfs[1] = xq * (1 - yq)
	cfs[2] = (1 - xq) * yq
	cfs[3] = xq * yq

	for i := range pxs {
		wa := pxs[i].a * cfs[i]
		px.r += pxs[i].r * wa
		px.g += pxs[i].g * wa
		px.b += pxs[i].b * wa
		px.a += wa
	}

	if px.a != 0 {
		px.r /= px.a
		px.g /= px.a
		px.b /= px.a
	}

	return px
}

func interpolateNearest(xf, yf float32, bounds image.Rectangle, pixGetter *pixelGetter, bgpx pixel) pixel {
	x0, y0 := int(floorf32(xf+0.5)), int(floorf32(yf+0.5))
	if image.Pt(x0, y0).In(bounds) {
		return pixGetter.getPixel(x0, y0)
	}
	return bgpx
}

// Rotate creates a filter that rotates an image by the given angle counter-clockwise.
// The angle parameter is the rotation angle in degrees.
// The backgroundColor parameter specifies the color of the uncovered zone after the rotation.
// The interpolation parameter specifies the interpolation method.
// Supported interpolation methods: NearestNeighborInterpolation, LinearInterpolation, CubicInterpolation.
//
// Example:
//
//	g := gift.New(
//		gift.Rotate(45, color.Black, gift.LinearInterpolation),
//	)
//	dst := image.NewRGBA(g.Bounds(src.Bounds()))
//	g.Draw(dst, src)
//
func Rotate(angle float32, backgroundColor color.Color, interpolation Interpolation) Filter {
	return &rotateFilter{
		angle:         angle,
		bgcolor:       backgroundColor,
		interpolation: interpolation,
	}
}

type cropFilter struct {
	rect image.Rectangle
}

func (p *cropFilter) Bounds(srcBounds image.Rectangle) (dstBounds image.Rectangle) {
	b := srcBounds.Intersect(p.rect)
	return b.Sub(b.Min)
}

func (p *cropFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	if options == nil {
		options = &defaultOptions
	}

	srcb := src.Bounds().Intersect(p.rect)
	dstb := dst.Bounds()
	pixGetter := newPixelGetter(src)
	pixSetter := newPixelSetter(dst)

	parallelize(options.Parallelization, srcb.Min.Y, srcb.Max.Y, func(pmin, pmax int) {
		for srcy := pmin; srcy < pmax; srcy++ {
			for srcx := srcb.Min.X; srcx < srcb.Max.X; srcx++ {
				dstx := dstb.Min.X + srcx - srcb.Min.X
				dsty := dstb.Min.Y + srcy - srcb.Min.Y
				pixSetter.setPixel(dstx, dsty, pixGetter.getPixel(srcx, srcy))
			}
		}
	})
}

// Crop creates a filter that crops the specified rectangular region from an image.
//
// Example:
//
//	g := gift.New(
//		gift.Crop(image.Rect(100, 100, 200, 200)),
//	)
//	dst := image.NewRGBA(g.Bounds(src.Bounds()))
//	g.Draw(dst, src)
//
func Crop(rect image.Rectangle) Filter {
	return &cropFilter{
		rect: rect,
	}
}

// Anchor is the anchor point for image cropping.
type Anchor int

// Anchor point positions.
const (
	CenterAnchor Anchor = iota
	TopLeftAnchor
	TopAnchor
	TopRightAnchor
	LeftAnchor
	RightAnchor
	BottomLeftAnchor
	BottomAnchor
	BottomRightAnchor
)

func anchorPt(b image.Rectangle, w, h int, anchor Anchor) image.Point {
	var x, y int
	switch anchor {
	case TopLeftAnchor:
		x = b.Min.X
		y = b.Min.Y
	case TopAnchor:
		x = b.Min.X + (b.Dx()-w)/2
		y = b.Min.Y
	case TopRightAnchor:
		x = b.Max.X - w
		y = b.Min.Y
	case LeftAnchor:
		x = b.Min.X
		y = b.Min.Y + (b.Dy()-h)/2
	case RightAnchor:
		x = b.Max.X - w
		y = b.Min.Y + (b.Dy()-h)/2
	case BottomLeftAnchor:
		x = b.Min.X
		y = b.Max.Y - h
	case BottomAnchor:
		x = b.Min.X + (b.Dx()-w)/2
		y = b.Max.Y - h
	case BottomRightAnchor:
		x = b.Max.X - w
		y = b.Max.Y - h
	default:
		x = b.Min.X + (b.Dx()-w)/2
		y = b.Min.Y + (b.Dy()-h)/2
	}
	return image.Pt(x, y)
}

type cropToSizeFilter struct {
	w, h   int
	anchor Anchor
}

func (p *cropToSizeFilter) Bounds(srcBounds image.Rectangle) (dstBounds image.Rectangle) {
	if p.w <= 0 || p.h <= 0 {
		return image.Rect(0, 0, 0, 0)
	}
	pt := anchorPt(srcBounds, p.w, p.h, p.anchor)
	r := image.Rect(0, 0, p.w, p.h).Add(pt)
	b := srcBounds.Intersect(r)
	return b.Sub(b.Min)
}

func (p *cropToSizeFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	if p.w <= 0 || p.h <= 0 {
		return
	}
	pt := anchorPt(src.Bounds(), p.w, p.h, p.anchor)
	r := image.Rect(0, 0, p.w, p.h).Add(pt)
	b := src.Bounds().Intersect(r)
	Crop(b).Draw(dst, src, options)
}

// CropToSize creates a filter that crops an image to the specified size using the specified anchor point.
func CropToSize(width, height int, anchor Anchor) Filter {
	return &cropToSizeFilter{
		w:      width,
		h:      height,
		anchor: anchor,
	}
}
