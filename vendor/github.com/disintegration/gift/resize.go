package gift

import (
	"image"
	"image/draw"
	"math"
)

// Resampling is an interpolation algorithm used for image resizing.
type Resampling interface {
	Support() float32
	Kernel(float32) float32
}

func bcspline(x, b, c float32) float32 {
	if x < 0 {
		x = -x
	}
	if x < 1 {
		return ((12-9*b-6*c)*x*x*x + (-18+12*b+6*c)*x*x + (6 - 2*b)) / 6
	}
	if x < 2 {
		return ((-b-6*c)*x*x*x + (6*b+30*c)*x*x + (-12*b-48*c)*x + (8*b + 24*c)) / 6
	}
	return 0
}

func sinc(x float32) float32 {
	if x == 0 {
		return 1
	}
	return float32(math.Sin(math.Pi*float64(x)) / (math.Pi * float64(x)))
}

type resamp struct {
	name    string
	support float32
	kernel  func(float32) float32
}

func (r resamp) String() string {
	return r.name
}

func (r resamp) Support() float32 {
	return r.support
}

func (r resamp) Kernel(x float32) float32 {
	return r.kernel(x)
}

// NearestNeighborResampling is a nearest neighbor resampling filter.
var NearestNeighborResampling Resampling

// BoxResampling is a box resampling filter (average of surrounding pixels).
var BoxResampling Resampling

// LinearResampling is a bilinear resampling filter.
var LinearResampling Resampling

// CubicResampling is a bicubic resampling filter (Catmull-Rom).
var CubicResampling Resampling

// LanczosResampling is a Lanczos resampling filter (3 lobes).
var LanczosResampling Resampling

type resampWeight struct {
	index  int
	weight float32
}

func prepareResampWeights(dstSize, srcSize int, resampling Resampling) [][]resampWeight {
	delta := float32(srcSize) / float32(dstSize)
	scale := delta
	if scale < 1 {
		scale = 1
	}
	radius := float32(math.Ceil(float64(scale * resampling.Support())))

	result := make([][]resampWeight, dstSize)
	tmp := make([]resampWeight, 0, dstSize*int(radius+2)*2)

	for i := 0; i < dstSize; i++ {
		center := (float32(i)+0.5)*delta - 0.5

		left := int(math.Ceil(float64(center - radius)))
		if left < 0 {
			left = 0
		}
		right := int(math.Floor(float64(center + radius)))
		if right > srcSize-1 {
			right = srcSize - 1
		}

		var sum float32
		for j := left; j <= right; j++ {
			weight := resampling.Kernel((float32(j) - center) / scale)
			if weight == 0 {
				continue
			}
			tmp = append(tmp, resampWeight{
				index:  j,
				weight: weight,
			})
			sum += weight
		}

		for j := range tmp {
			tmp[j].weight /= sum
		}

		result[i] = tmp
		tmp = tmp[len(tmp):]
	}

	return result
}

func resizeLine(dst []pixel, src []pixel, weights [][]resampWeight) {
	for i := 0; i < len(dst); i++ {
		var r, g, b, a float32
		for _, w := range weights[i] {
			c := src[w.index]
			wa := c.a * w.weight
			r += c.r * wa
			g += c.g * wa
			b += c.b * wa
			a += wa
		}
		if a != 0 {
			r /= a
			g /= a
			b /= a
		}
		dst[i] = pixel{r, g, b, a}
	}
}

func resizeHorizontal(dst draw.Image, src image.Image, w int, resampling Resampling, options *Options) {
	srcb := src.Bounds()
	dstb := dst.Bounds()

	weights := prepareResampWeights(w, srcb.Dx(), resampling)

	pixGetter := newPixelGetter(src)
	pixSetter := newPixelSetter(dst)

	parallelize(options.Parallelization, srcb.Min.Y, srcb.Max.Y, func(pmin, pmax int) {
		srcBuf := make([]pixel, srcb.Dx())
		dstBuf := make([]pixel, w)
		for srcy := pmin; srcy < pmax; srcy++ {
			pixGetter.getPixelRow(srcy, &srcBuf)
			resizeLine(dstBuf, srcBuf, weights)
			pixSetter.setPixelRow(dstb.Min.Y+srcy-srcb.Min.Y, dstBuf)
		}
	})
}

func resizeVertical(dst draw.Image, src image.Image, h int, resampling Resampling, options *Options) {
	srcb := src.Bounds()
	dstb := dst.Bounds()

	weights := prepareResampWeights(h, srcb.Dy(), resampling)

	pixGetter := newPixelGetter(src)
	pixSetter := newPixelSetter(dst)

	parallelize(options.Parallelization, srcb.Min.X, srcb.Max.X, func(pmin, pmax int) {
		srcBuf := make([]pixel, srcb.Dy())
		dstBuf := make([]pixel, h)
		for srcx := pmin; srcx < pmax; srcx++ {
			pixGetter.getPixelColumn(srcx, &srcBuf)
			resizeLine(dstBuf, srcBuf, weights)
			pixSetter.setPixelColumn(dstb.Min.X+srcx-srcb.Min.X, dstBuf)
		}
	})
}

func resizeNearest(dst draw.Image, src image.Image, w, h int, options *Options) {
	srcb := src.Bounds()
	dstb := dst.Bounds()
	dx := float64(srcb.Dx()) / float64(w)
	dy := float64(srcb.Dy()) / float64(h)

	pixGetter := newPixelGetter(src)
	pixSetter := newPixelSetter(dst)

	parallelize(options.Parallelization, dstb.Min.Y, dstb.Min.Y+h, func(pmin, pmax int) {
		for dsty := pmin; dsty < pmax; dsty++ {
			for dstx := dstb.Min.X; dstx < dstb.Min.X+w; dstx++ {
				fx := math.Floor((float64(dstx-dstb.Min.X) + 0.5) * dx)
				fy := math.Floor((float64(dsty-dstb.Min.Y) + 0.5) * dy)
				srcx := srcb.Min.X + int(fx)
				srcy := srcb.Min.Y + int(fy)
				px := pixGetter.getPixel(srcx, srcy)
				pixSetter.setPixel(dstx, dsty, px)
			}
		}
	})
}

type resizeFilter struct {
	width      int
	height     int
	resampling Resampling
}

func (p *resizeFilter) Bounds(srcBounds image.Rectangle) (dstBounds image.Rectangle) {
	w, h := p.width, p.height
	srcw, srch := srcBounds.Dx(), srcBounds.Dy()

	if (w == 0 && h == 0) || w < 0 || h < 0 || srcw <= 0 || srch <= 0 {
		dstBounds = image.Rect(0, 0, 0, 0)
	} else if w == 0 {
		fw := float64(h) * float64(srcw) / float64(srch)
		dstw := int(math.Max(1, math.Floor(fw+0.5)))
		dstBounds = image.Rect(0, 0, dstw, h)
	} else if h == 0 {
		fh := float64(w) * float64(srch) / float64(srcw)
		dsth := int(math.Max(1, math.Floor(fh+0.5)))
		dstBounds = image.Rect(0, 0, w, dsth)
	} else {
		dstBounds = image.Rect(0, 0, w, h)
	}

	return
}

func (p *resizeFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	if options == nil {
		options = &defaultOptions
	}

	b := p.Bounds(src.Bounds())
	w, h := b.Dx(), b.Dy()

	if w <= 0 || h <= 0 {
		return
	}

	if src.Bounds().Dx() == w && src.Bounds().Dy() == h {
		copyimage(dst, src, options)
		return
	}

	if p.resampling.Support() <= 0 {
		resizeNearest(dst, src, w, h, options)
		return
	}

	if src.Bounds().Dx() == w {
		resizeVertical(dst, src, h, p.resampling, options)
		return
	}

	if src.Bounds().Dy() == h {
		resizeHorizontal(dst, src, w, p.resampling, options)
		return
	}

	tmp := createTempImage(image.Rect(0, 0, w, src.Bounds().Dy()))
	resizeHorizontal(tmp, src, w, p.resampling, options)
	resizeVertical(dst, tmp, h, p.resampling, options)
}

// Resize creates a filter that resizes an image to the specified width and height using the specified resampling.
// If one of width or height is 0, the image aspect ratio is preserved.
// Supported resampling parameters: NearestNeighborResampling, BoxResampling, LinearResampling, CubicResampling, LanczosResampling.
//
// Example:
//
//	// Resize the src image to width=300 preserving the aspect ratio.
//	g := gift.New(
//		gift.Resize(300, 0, gift.LanczosResampling),
//	)
//	dst := image.NewRGBA(g.Bounds(src.Bounds()))
//	g.Draw(dst, src)
//
func Resize(width, height int, resampling Resampling) Filter {
	return &resizeFilter{
		width:      width,
		height:     height,
		resampling: resampling,
	}
}

type resizeToFitFilter struct {
	width      int
	height     int
	resampling Resampling
}

func (p *resizeToFitFilter) Bounds(srcBounds image.Rectangle) image.Rectangle {
	w, h := p.width, p.height
	srcw, srch := srcBounds.Dx(), srcBounds.Dy()

	if w <= 0 || h <= 0 || srcw <= 0 || srch <= 0 {
		return image.Rect(0, 0, 0, 0)
	}

	if srcw <= w && srch <= h {
		return image.Rect(0, 0, srcw, srch)
	}

	wratio := float64(srcw) / float64(w)
	hratio := float64(srch) / float64(h)

	var dstw, dsth int
	if wratio > hratio {
		dstw = w
		dsth = minint(int(float64(srch)/wratio+0.5), h)
	} else {
		dsth = h
		dstw = minint(int(float64(srcw)/hratio+0.5), w)
	}

	return image.Rect(0, 0, dstw, dsth)
}

func (p *resizeToFitFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	b := p.Bounds(src.Bounds())
	Resize(b.Dx(), b.Dy(), p.resampling).Draw(dst, src, options)
}

// ResizeToFit creates a filter that resizes an image to fit within the specified dimensions while preserving the aspect ratio.
// Supported resampling parameters: NearestNeighborResampling, BoxResampling, LinearResampling, CubicResampling, LanczosResampling.
func ResizeToFit(width, height int, resampling Resampling) Filter {
	return &resizeToFitFilter{
		width:      width,
		height:     height,
		resampling: resampling,
	}
}

type resizeToFillFilter struct {
	width      int
	height     int
	anchor     Anchor
	resampling Resampling
}

func (p *resizeToFillFilter) Bounds(srcBounds image.Rectangle) image.Rectangle {
	w, h := p.width, p.height
	srcw, srch := srcBounds.Dx(), srcBounds.Dy()

	if w <= 0 || h <= 0 || srcw <= 0 || srch <= 0 {
		return image.Rect(0, 0, 0, 0)
	}

	return image.Rect(0, 0, w, h)
}

func (p *resizeToFillFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	b := p.Bounds(src.Bounds())
	w, h := b.Dx(), b.Dy()

	if w <= 0 || h <= 0 {
		return
	}

	srcw, srch := src.Bounds().Dx(), src.Bounds().Dy()

	wratio := float64(srcw) / float64(w)
	hratio := float64(srch) / float64(h)

	var tmpw, tmph int
	if wratio < hratio {
		tmpw = w
		tmph = maxint(int(float64(srch)/wratio+0.5), h)
	} else {
		tmph = h
		tmpw = maxint(int(float64(srcw)/hratio+0.5), w)
	}

	tmp := createTempImage(image.Rect(0, 0, tmpw, tmph))
	Resize(tmpw, tmph, p.resampling).Draw(tmp, src, options)
	CropToSize(w, h, p.anchor).Draw(dst, tmp, options)
}

// ResizeToFill creates a filter that resizes an image to the smallest possible size that will cover the specified dimensions,
// then crops the resized image to the specified dimensions using the specified anchor point.
// Supported resampling parameters: NearestNeighborResampling, BoxResampling, LinearResampling, CubicResampling, LanczosResampling.
func ResizeToFill(width, height int, resampling Resampling, anchor Anchor) Filter {
	return &resizeToFillFilter{
		width:      width,
		height:     height,
		anchor:     anchor,
		resampling: resampling,
	}
}

func init() {
	// Nearest neighbor resampling filter.
	NearestNeighborResampling = resamp{
		name:    "NearestNeighborResampling",
		support: 0,
		kernel: func(x float32) float32 {
			return 0
		},
	}

	// Box resampling filter.
	BoxResampling = resamp{
		name:    "BoxResampling",
		support: 0.5,
		kernel: func(x float32) float32 {
			if x < 0 {
				x = -x
			}
			if x <= 0.5 {
				return 1
			}
			return 0
		},
	}

	// Linear resampling filter.
	LinearResampling = resamp{
		name:    "LinearResampling",
		support: 1,
		kernel: func(x float32) float32 {
			if x < 0 {
				x = -x
			}
			if x < 1 {
				return 1 - x
			}
			return 0
		},
	}

	// Cubic resampling filter (Catmull-Rom).
	CubicResampling = resamp{
		name:    "CubicResampling",
		support: 2,
		kernel: func(x float32) float32 {
			if x < 0 {
				x = -x
			}
			if x < 2 {
				return bcspline(x, 0, 0.5)
			}
			return 0
		},
	}

	// Lanczos resampling filter (3 lobes).
	LanczosResampling = resamp{
		name:    "LanczosResampling",
		support: 3,
		kernel: func(x float32) float32 {
			if x < 0 {
				x = -x
			}
			if x < 3 {
				return sinc(x) * sinc(x/3)
			}
			return 0
		},
	}
}
