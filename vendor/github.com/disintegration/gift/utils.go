package gift

import (
	"image"
	"image/draw"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
)

// parallelize parallelizes the data processing if enabled is true.
func parallelize(enabled bool, datamin, datamax int, fn func(pmin, pmax int)) {
	datasize := datamax - datamin
	partsize := datasize

	numGoroutines := 1
	if enabled {
		numProcs := runtime.GOMAXPROCS(0)
		if numProcs > 1 {
			numGoroutines = numProcs
			partsize = partsize / numGoroutines
			if partsize < 1 {
				partsize = 1
			}
		}
	}

	if numGoroutines == 1 {
		fn(datamin, datamax)
	} else {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		idx := int64(datamin)

		for p := 0; p < numGoroutines; p++ {
			go func() {
				defer wg.Done()
				for {
					pmin := int(atomic.AddInt64(&idx, int64(partsize))) - partsize
					if pmin >= datamax {
						break
					}
					pmax := pmin + partsize
					if pmax > datamax {
						pmax = datamax
					}
					fn(pmin, pmax)
				}
			}()
		}

		wg.Wait()
	}
}

func absf32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

func minf32(x, y float32) float32 {
	if x < y {
		return x
	}
	return y
}

func maxf32(x, y float32) float32 {
	if x > y {
		return x
	}
	return y
}

func powf32(x, y float32) float32 {
	return float32(math.Pow(float64(x), float64(y)))
}

func logf32(x float32) float32 {
	return float32(math.Log(float64(x)))
}

func expf32(x float32) float32 {
	return float32(math.Exp(float64(x)))
}

func sincosf32(a float32) (float32, float32) {
	sin, cos := math.Sincos(math.Pi * float64(a) / 180)
	return float32(sin), float32(cos)
}

func floorf32(x float32) float32 {
	return float32(math.Floor(float64(x)))
}

func sqrtf32(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}

func minint(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func maxint(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func sort(data []float32) {
	n := len(data)

	if n < 2 {
		return
	}

	if n <= 20 {
		for i := 1; i < n; i++ {
			x := data[i]
			j := i - 1
			for ; j >= 0 && data[j] > x; j-- {
				data[j+1] = data[j]
			}
			data[j+1] = x
		}
		return
	}

	i := 0
	j := n - 1
	x := data[n/2]
	for i <= j {
		for data[i] < x {
			i++
		}
		for data[j] > x {
			j--
		}
		if i <= j {
			data[i], data[j] = data[j], data[i]
			i++
			j--
		}
	}
	if j > 0 {
		sort(data[:j+1])
	}
	if i < n-1 {
		sort(data[i:])
	}
}

// createTempImage creates a temporary image.
func createTempImage(r image.Rectangle) draw.Image {
	return image.NewNRGBA64(r)
}

// isOpaque checks if the given image is opaque.
func isOpaque(img image.Image) bool {
	switch img := img.(type) {
	case *image.NRGBA:
		return img.Opaque()
	case *image.NRGBA64:
		return img.Opaque()
	case *image.RGBA:
		return img.Opaque()
	case *image.RGBA64:
		return img.Opaque()
	case *image.Gray:
		return true
	case *image.Gray16:
		return true
	case *image.YCbCr:
		return true
	case *image.Paletted:
		return img.Opaque()
	}
	return false
}

// genDisk generates a disk-shaped kernel.
func genDisk(ksize int) []float32 {
	if ksize%2 == 0 {
		ksize--
	}
	if ksize < 1 {
		return []float32{}
	}
	disk := make([]float32, ksize*ksize)
	kcenter := ksize / 2
	for i := 0; i < ksize; i++ {
		for j := 0; j < ksize; j++ {
			x := kcenter - i
			y := kcenter - j
			r := math.Sqrt(float64(x*x + y*y))
			if r <= float64(ksize/2) {
				disk[j*ksize+i] = 1
			}
		}
	}
	return disk
}

// copyimage copies an image from src to dst.
func copyimage(dst draw.Image, src image.Image, options *Options) {
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
				dstx := dstb.Min.X + srcx - srcb.Min.X
				dsty := dstb.Min.Y + srcy - srcb.Min.Y
				pixSetter.setPixel(dstx, dsty, pixGetter.getPixel(srcx, srcy))
			}
		}
	})
}

type copyimageFilter struct{}

func (p *copyimageFilter) Bounds(srcBounds image.Rectangle) (dstBounds image.Rectangle) {
	dstBounds = image.Rect(0, 0, srcBounds.Dx(), srcBounds.Dy())
	return
}

func (p *copyimageFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	copyimage(dst, src, options)
}
