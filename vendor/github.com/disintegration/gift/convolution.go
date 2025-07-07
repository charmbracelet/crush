package gift

import (
	"image"
	"image/draw"
	"math"
)

type uweight struct {
	u      int
	weight float32
}

type uvweight struct {
	u      int
	v      int
	weight float32
}

func prepareConvolutionWeights(kernel []float32, normalize bool) (int, []uvweight) {
	size := int(math.Sqrt(float64(len(kernel))))
	if size%2 == 0 {
		size--
	}
	if size < 1 {
		return 0, []uvweight{}
	}
	center := size / 2

	weights := []uvweight{}
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			k := j*size + i
			w := float32(0)
			if k < len(kernel) {
				w = kernel[k]
			}
			if w != 0 {
				weights = append(weights, uvweight{u: i - center, v: j - center, weight: w})
			}
		}
	}

	if !normalize {
		return size, weights
	}

	var sum, sumpositive float32
	for _, w := range weights {
		sum += w.weight
		if w.weight > 0 {
			sumpositive += w.weight
		}
	}

	var div float32
	if sum != 0 {
		div = sum
	} else if sumpositive != 0 {
		div = sumpositive
	} else {
		return size, weights
	}

	for i := 0; i < len(weights); i++ {
		weights[i].weight /= div
	}

	return size, weights
}

type convolutionFilter struct {
	kernel    []float32
	normalize bool
	alpha     bool
	abs       bool
	delta     float32
}

func (p *convolutionFilter) Bounds(srcBounds image.Rectangle) (dstBounds image.Rectangle) {
	dstBounds = image.Rect(0, 0, srcBounds.Dx(), srcBounds.Dy())
	return
}

func (p *convolutionFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	if options == nil {
		options = &defaultOptions
	}

	srcb := src.Bounds()
	dstb := dst.Bounds()

	if srcb.Dx() <= 0 || srcb.Dy() <= 0 {
		return
	}

	ksize, weights := prepareConvolutionWeights(p.kernel, p.normalize)
	kcenter := ksize / 2

	if ksize < 1 {
		copyimage(dst, src, options)
		return
	}

	pixGetter := newPixelGetter(src)
	pixSetter := newPixelSetter(dst)

	parallelize(options.Parallelization, srcb.Min.Y, srcb.Max.Y, func(pmin, pmax int) {
		// init temp rows
		starty := pmin
		rows := make([][]pixel, ksize)
		for i := 0; i < ksize; i++ {
			rowy := starty + i - kcenter
			if rowy < srcb.Min.Y {
				rowy = srcb.Min.Y
			} else if rowy > srcb.Max.Y-1 {
				rowy = srcb.Max.Y - 1
			}
			row := make([]pixel, srcb.Dx())
			pixGetter.getPixelRow(rowy, &row)
			rows[i] = row
		}

		for y := pmin; y < pmax; y++ {
			// calculate dst row
			for x := srcb.Min.X; x < srcb.Max.X; x++ {
				var r, g, b, a float32
				for _, w := range weights {
					wx := x + w.u
					if wx < srcb.Min.X {
						wx = srcb.Min.X
					} else if wx > srcb.Max.X-1 {
						wx = srcb.Max.X - 1
					}
					rowsx := wx - srcb.Min.X
					rowsy := kcenter + w.v

					px := rows[rowsy][rowsx]
					r += px.r * w.weight
					g += px.g * w.weight
					b += px.b * w.weight
					if p.alpha {
						a += px.a * w.weight
					}
				}
				if p.abs {
					r = absf32(r)
					g = absf32(g)
					b = absf32(b)
					if p.alpha {
						a = absf32(a)
					}
				}
				if p.delta != 0 {
					r += p.delta
					g += p.delta
					b += p.delta
					if p.alpha {
						a += p.delta
					}
				}
				if !p.alpha {
					a = rows[kcenter][x-srcb.Min.X].a
				}
				pixSetter.setPixel(dstb.Min.X+x-srcb.Min.X, dstb.Min.Y+y-srcb.Min.Y, pixel{r, g, b, a})
			}

			// rotate temp rows
			if y < pmax-1 {
				tmprow := rows[0]
				for i := 0; i < ksize-1; i++ {
					rows[i] = rows[i+1]
				}
				nextrowy := y + ksize/2 + 1
				if nextrowy > srcb.Max.Y-1 {
					nextrowy = srcb.Max.Y - 1
				}
				pixGetter.getPixelRow(nextrowy, &tmprow)
				rows[ksize-1] = tmprow
			}
		}
	})
}

// Convolution creates a filter that applies a square convolution kernel to an image.
// The length of the kernel slice must be the square of an odd kernel size (e.g. 9 for 3x3 kernel, 25 for 5x5 kernel).
// Excessive slice members will be ignored.
// If normalize parameter is true, the kernel will be normalized before applying the filter.
// If alpha parameter is true, the alpha component of color will be filtered too.
// If abs parameter is true, absolute values of color components will be taken after doing calculations.
// If delta parameter is not zero, this value will be added to the filtered pixels.
//
// Example:
//
//	// Apply the emboss filter to an image.
//	g := gift.New(
//		gift.Convolution(
//			[]float32{
//				-1, -1, 0,
//				-1, 1, 1,
//				0, 1, 1,
//			},
//			false, false, false, 0,
//		),
//	)
//	dst := image.NewRGBA(g.Bounds(src.Bounds()))
//	g.Draw(dst, src)
//
func Convolution(kernel []float32, normalize, alpha, abs bool, delta float32) Filter {
	return &convolutionFilter{
		kernel:    kernel,
		normalize: normalize,
		alpha:     alpha,
		abs:       abs,
		delta:     delta,
	}
}

// prepare pixel weights using convolution kernel. weights equal to 0 are excluded
func prepareConvolutionWeights1d(kernel []float32) (int, []uweight) {
	size := len(kernel)
	if size%2 == 0 {
		size--
	}
	if size < 1 {
		return 0, []uweight{}
	}
	center := size / 2
	weights := []uweight{}
	for i := 0; i < size; i++ {
		w := float32(0)
		if i < len(kernel) {
			w = kernel[i]
		}
		if w != 0 {
			weights = append(weights, uweight{i - center, w})
		}
	}
	return size, weights
}

// calculate pixels for one line according to weights
func convolveLine(dstBuf []pixel, srcBuf []pixel, weights []uweight) {
	max := len(srcBuf) - 1
	if max < 0 {
		return
	}
	for dstu := 0; dstu < len(srcBuf); dstu++ {
		var r, g, b, a float32
		for _, w := range weights {
			k := dstu + w.u
			if k < 0 {
				k = 0
			} else if k > max {
				k = max
			}
			c := srcBuf[k]
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
		dstBuf[dstu] = pixel{r, g, b, a}
	}
}

// fast vertical 1d convolution
func convolve1dv(dst draw.Image, src image.Image, kernel []float32, options *Options) {
	srcb := src.Bounds()
	dstb := dst.Bounds()
	if srcb.Dx() <= 0 || srcb.Dy() <= 0 {
		return
	}
	if kernel == nil || len(kernel) < 1 {
		copyimage(dst, src, options)
		return
	}
	_, weights := prepareConvolutionWeights1d(kernel)
	pixGetter := newPixelGetter(src)
	pixSetter := newPixelSetter(dst)
	parallelize(options.Parallelization, srcb.Min.X, srcb.Max.X, func(pmin, pmax int) {
		srcBuf := make([]pixel, srcb.Dy())
		dstBuf := make([]pixel, srcb.Dy())
		for x := pmin; x < pmax; x++ {
			pixGetter.getPixelColumn(x, &srcBuf)
			convolveLine(dstBuf, srcBuf, weights)
			pixSetter.setPixelColumn(dstb.Min.X+x-srcb.Min.X, dstBuf)
		}
	})
}

// fast horizontal 1d convolution
func convolve1dh(dst draw.Image, src image.Image, kernel []float32, options *Options) {
	srcb := src.Bounds()
	dstb := dst.Bounds()
	if srcb.Dx() <= 0 || srcb.Dy() <= 0 {
		return
	}
	if kernel == nil || len(kernel) < 1 {
		copyimage(dst, src, options)
		return
	}
	_, weights := prepareConvolutionWeights1d(kernel)
	pixGetter := newPixelGetter(src)
	pixSetter := newPixelSetter(dst)
	parallelize(options.Parallelization, srcb.Min.Y, srcb.Max.Y, func(pmin, pmax int) {
		srcBuf := make([]pixel, srcb.Dx())
		dstBuf := make([]pixel, srcb.Dx())
		for y := pmin; y < pmax; y++ {
			pixGetter.getPixelRow(y, &srcBuf)
			convolveLine(dstBuf, srcBuf, weights)
			pixSetter.setPixelRow(dstb.Min.Y+y-srcb.Min.Y, dstBuf)
		}
	})
}

func gaussianBlurKernel(x, sigma float32) float32 {
	return float32(math.Exp(-float64(x*x)/float64(2*sigma*sigma)) / (float64(sigma) * math.Sqrt(2*math.Pi)))
}

type gausssianBlurFilter struct {
	sigma float32
}

func (p *gausssianBlurFilter) Bounds(srcBounds image.Rectangle) (dstBounds image.Rectangle) {
	dstBounds = image.Rect(0, 0, srcBounds.Dx(), srcBounds.Dy())
	return
}

func (p *gausssianBlurFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	if options == nil {
		options = &defaultOptions
	}

	srcb := src.Bounds()
	if srcb.Dx() <= 0 || srcb.Dy() <= 0 {
		return
	}

	if p.sigma <= 0 {
		copyimage(dst, src, options)
		return
	}

	radius := int(math.Ceil(float64(p.sigma * 3)))
	size := 2*radius + 1
	center := radius
	kernel := make([]float32, size)

	kernel[center] = gaussianBlurKernel(0, p.sigma)
	sum := kernel[center]

	for i := 1; i <= radius; i++ {
		f := gaussianBlurKernel(float32(i), p.sigma)
		kernel[center-i] = f
		kernel[center+i] = f
		sum += 2 * f
	}

	for i := 0; i < len(kernel); i++ {
		kernel[i] /= sum
	}

	tmp := createTempImage(srcb)
	convolve1dh(tmp, src, kernel, options)
	convolve1dv(dst, tmp, kernel, options)
}

// GaussianBlur creates a filter that applies a gaussian blur to an image.
// The sigma parameter must be positive and indicates how much the image will be blurred.
// Blur affected radius roughly equals 3 * sigma.
//
// Example:
//
//	g := gift.New(
//		gift.GaussianBlur(1.5),
//	)
//	dst := image.NewRGBA(g.Bounds(src.Bounds()))
//	g.Draw(dst, src)
//
func GaussianBlur(sigma float32) Filter {
	return &gausssianBlurFilter{
		sigma: sigma,
	}
}

type unsharpMaskFilter struct {
	sigma     float32
	amount    float32
	threshold float32
}

func (p *unsharpMaskFilter) Bounds(srcBounds image.Rectangle) (dstBounds image.Rectangle) {
	dstBounds = image.Rect(0, 0, srcBounds.Dx(), srcBounds.Dy())
	return
}

func unsharp(orig, blurred, amount, threshold float32) float32 {
	dif := (orig - blurred) * amount
	if absf32(dif) > absf32(threshold) {
		return orig + dif
	}
	return orig
}

func (p *unsharpMaskFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	if options == nil {
		options = &defaultOptions
	}

	srcb := src.Bounds()
	dstb := dst.Bounds()

	if srcb.Dx() <= 0 || srcb.Dy() <= 0 {
		return
	}

	blurred := createTempImage(srcb)
	blur := GaussianBlur(p.sigma)
	blur.Draw(blurred, src, options)

	pixGetterOrig := newPixelGetter(src)
	pixGetterBlur := newPixelGetter(blurred)
	pixelSetter := newPixelSetter(dst)

	parallelize(options.Parallelization, srcb.Min.Y, srcb.Max.Y, func(pmin, pmax int) {
		for y := pmin; y < pmax; y++ {
			for x := srcb.Min.X; x < srcb.Max.X; x++ {
				pxOrig := pixGetterOrig.getPixel(x, y)
				pxBlur := pixGetterBlur.getPixel(x, y)

				r := unsharp(pxOrig.r, pxBlur.r, p.amount, p.threshold)
				g := unsharp(pxOrig.g, pxBlur.g, p.amount, p.threshold)
				b := unsharp(pxOrig.b, pxBlur.b, p.amount, p.threshold)
				a := unsharp(pxOrig.a, pxBlur.a, p.amount, p.threshold)

				pixelSetter.setPixel(dstb.Min.X+x-srcb.Min.X, dstb.Min.Y+y-srcb.Min.Y, pixel{r, g, b, a})
			}
		}
	})
}

// UnsharpMask creates a filter that sharpens an image.
// The sigma parameter is used in a gaussian function and affects the radius of effect.
// Sigma must be positive. Sharpen radius roughly equals 3 * sigma.
// The amount parameter controls how much darker and how much lighter the edge borders become. Typically between 0.5 and 1.5.
// The threshold parameter controls the minimum brightness change that will be sharpened. Typically between 0 and 0.05.
//
// Example:
//
//	g := gift.New(
//		gift.UnsharpMask(1, 1, 0),
//	)
//	dst := image.NewRGBA(g.Bounds(src.Bounds()))
//	g.Draw(dst, src)
//
func UnsharpMask(sigma, amount, threshold float32) Filter {
	return &unsharpMaskFilter{
		sigma:     sigma,
		amount:    amount,
		threshold: threshold,
	}
}

type meanFilter struct {
	ksize int
	disk  bool
}

func (p *meanFilter) Bounds(srcBounds image.Rectangle) (dstBounds image.Rectangle) {
	dstBounds = image.Rect(0, 0, srcBounds.Dx(), srcBounds.Dy())
	return
}

func (p *meanFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	if options == nil {
		options = &defaultOptions
	}

	srcb := src.Bounds()
	if srcb.Dx() <= 0 || srcb.Dy() <= 0 {
		return
	}

	ksize := p.ksize
	if ksize%2 == 0 {
		ksize--
	}

	if ksize <= 1 {
		copyimage(dst, src, options)
		return
	}

	if p.disk {
		diskKernel := genDisk(p.ksize)
		f := Convolution(diskKernel, true, true, false, 0)
		f.Draw(dst, src, options)
	} else {
		kernel := make([]float32, ksize*ksize)
		for i := range kernel {
			kernel[i] = 1
		}
		f := Convolution(kernel, true, true, false, 0)
		f.Draw(dst, src, options)
	}
}

// Mean creates a local mean image filter.
// Takes an average across a neighborhood for each pixel.
// The ksize parameter is the kernel size. It must be an odd positive integer (for example: 3, 5, 7).
// If the disk parameter is true, a disk-shaped neighborhood will be used instead of a square neighborhood.
func Mean(ksize int, disk bool) Filter {
	return &meanFilter{
		ksize: ksize,
		disk:  disk,
	}
}

type hvConvolutionFilter struct {
	hkernel, vkernel []float32
}

func (p *hvConvolutionFilter) Bounds(srcBounds image.Rectangle) (dstBounds image.Rectangle) {
	dstBounds = image.Rect(0, 0, srcBounds.Dx(), srcBounds.Dy())
	return
}

func (p *hvConvolutionFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	if options == nil {
		options = &defaultOptions
	}

	srcb := src.Bounds()
	dstb := dst.Bounds()

	if srcb.Dx() <= 0 || srcb.Dy() <= 0 {
		return
	}

	tmph := createTempImage(srcb)
	Convolution(p.hkernel, false, false, true, 0).Draw(tmph, src, options)
	pixGetterH := newPixelGetter(tmph)

	tmpv := createTempImage(srcb)
	Convolution(p.vkernel, false, false, true, 0).Draw(tmpv, src, options)
	pixGetterV := newPixelGetter(tmpv)

	pixSetter := newPixelSetter(dst)

	parallelize(options.Parallelization, srcb.Min.Y, srcb.Max.Y, func(pmin, pmax int) {
		for y := pmin; y < pmax; y++ {
			for x := srcb.Min.X; x < srcb.Max.X; x++ {
				pxh := pixGetterH.getPixel(x, y)
				pxv := pixGetterV.getPixel(x, y)
				r := sqrtf32(pxh.r*pxh.r + pxv.r*pxv.r)
				g := sqrtf32(pxh.g*pxh.g + pxv.g*pxv.g)
				b := sqrtf32(pxh.b*pxh.b + pxv.b*pxv.b)
				pixSetter.setPixel(dstb.Min.X+x-srcb.Min.X, dstb.Min.Y+y-srcb.Min.Y, pixel{r, g, b, pxh.a})
			}
		}
	})

}

// Sobel creates a filter that applies a sobel operator to an image.
func Sobel() Filter {
	return &hvConvolutionFilter{
		hkernel: []float32{-1, 0, 1, -2, 0, 2, -1, 0, 1},
		vkernel: []float32{-1, -2, -1, 0, 0, 0, 1, 2, 1},
	}
}
