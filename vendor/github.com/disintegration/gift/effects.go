package gift

import (
	"image"
	"image/draw"
)

type pixelateFilter struct {
	size int
}

func (p *pixelateFilter) Bounds(srcBounds image.Rectangle) (dstBounds image.Rectangle) {
	dstBounds = image.Rect(0, 0, srcBounds.Dx(), srcBounds.Dy())
	return
}

func (p *pixelateFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	if options == nil {
		options = &defaultOptions
	}

	blockSize := p.size
	if blockSize <= 1 {
		copyimage(dst, src, options)
		return
	}

	srcb := src.Bounds()
	dstb := dst.Bounds()

	numBlocksX := srcb.Dx() / blockSize
	if srcb.Dx()%blockSize > 0 {
		numBlocksX++
	}
	numBlocksY := srcb.Dy() / blockSize
	if srcb.Dy()%blockSize > 0 {
		numBlocksY++
	}

	pixGetter := newPixelGetter(src)
	pixSetter := newPixelSetter(dst)

	parallelize(options.Parallelization, 0, numBlocksY, func(bmin, bmax int) {
		for by := bmin; by < bmax; by++ {
			for bx := 0; bx < numBlocksX; bx++ {
				// calculate the block bounds
				bb := image.Rect(bx*blockSize, by*blockSize, (bx+1)*blockSize, (by+1)*blockSize)
				bbSrc := bb.Add(srcb.Min).Intersect(srcb)
				bbDst := bbSrc.Sub(srcb.Min).Add(dstb.Min).Intersect(dstb)

				// calculate average color of the block
				var r, g, b, a float32
				var cnt float32
				for y := bbSrc.Min.Y; y < bbSrc.Max.Y; y++ {
					for x := bbSrc.Min.X; x < bbSrc.Max.X; x++ {
						px := pixGetter.getPixel(x, y)
						r += px.r
						g += px.g
						b += px.b
						a += px.a
						cnt++
					}
				}
				if cnt > 0 {
					r /= cnt
					g /= cnt
					b /= cnt
					a /= cnt
				}

				// set the calculated color for all pixels in the block
				for y := bbDst.Min.Y; y < bbDst.Max.Y; y++ {
					for x := bbDst.Min.X; x < bbDst.Max.X; x++ {
						pixSetter.setPixel(x, y, pixel{r, g, b, a})
					}
				}
			}
		}
	})
}

// Pixelate creates a filter that applies a pixelation effect to an image.
func Pixelate(size int) Filter {
	return &pixelateFilter{
		size: size,
	}
}
