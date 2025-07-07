package gift

import (
	"image"
	"image/draw"
)

type rankMode int

const (
	rankMedian rankMode = iota
	rankMin
	rankMax
)

type rankFilter struct {
	ksize int
	disk  bool
	mode  rankMode
}

func (p *rankFilter) Bounds(srcBounds image.Rectangle) (dstBounds image.Rectangle) {
	dstBounds = image.Rect(0, 0, srcBounds.Dx(), srcBounds.Dy())
	return
}

func (p *rankFilter) Draw(dst draw.Image, src image.Image, options *Options) {
	if options == nil {
		options = &defaultOptions
	}

	srcb := src.Bounds()
	dstb := dst.Bounds()

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
	kradius := ksize / 2

	opaque := isOpaque(src)

	var disk []float32
	if p.disk {
		disk = genDisk(ksize)
	}

	pixGetter := newPixelGetter(src)
	pixSetter := newPixelSetter(dst)

	parallelize(options.Parallelization, srcb.Min.Y, srcb.Max.Y, func(pmin, pmax int) {
		pxbuf := []pixel{}

		var rbuf, gbuf, bbuf, abuf []float32
		if p.mode == rankMedian {
			rbuf = make([]float32, 0, ksize*ksize)
			gbuf = make([]float32, 0, ksize*ksize)
			bbuf = make([]float32, 0, ksize*ksize)
			if !opaque {
				abuf = make([]float32, 0, ksize*ksize)
			}
		}

		for y := pmin; y < pmax; y++ {
			// init buffer
			pxbuf = pxbuf[:0]
			for i := srcb.Min.X - kradius; i <= srcb.Min.X+kradius; i++ {
				for j := y - kradius; j <= y+kradius; j++ {
					kx, ky := i, j
					if kx < srcb.Min.X {
						kx = srcb.Min.X
					} else if kx > srcb.Max.X-1 {
						kx = srcb.Max.X - 1
					}
					if ky < srcb.Min.Y {
						ky = srcb.Min.Y
					} else if ky > srcb.Max.Y-1 {
						ky = srcb.Max.Y - 1
					}
					pxbuf = append(pxbuf, pixGetter.getPixel(kx, ky))
				}
			}

			for x := srcb.Min.X; x < srcb.Max.X; x++ {
				var r, g, b, a float32
				if p.mode == rankMedian {
					rbuf = rbuf[:0]
					gbuf = gbuf[:0]
					bbuf = bbuf[:0]
					if !opaque {
						abuf = abuf[:0]
					}
				} else if p.mode == rankMin {
					r, g, b, a = 1, 1, 1, 1
				} else if p.mode == rankMax {
					r, g, b, a = 0, 0, 0, 0
				}

				sz := 0
				for i := 0; i < ksize; i++ {
					for j := 0; j < ksize; j++ {

						if p.disk {
							if disk[i*ksize+j] == 0 {
								continue
							}
						}

						px := pxbuf[i*ksize+j]
						if p.mode == rankMedian {
							rbuf = append(rbuf, px.r)
							gbuf = append(gbuf, px.g)
							bbuf = append(bbuf, px.b)
							if !opaque {
								abuf = append(abuf, px.a)
							}
						} else if p.mode == rankMin {
							r = minf32(r, px.r)
							g = minf32(g, px.g)
							b = minf32(b, px.b)
							if !opaque {
								a = minf32(a, px.a)
							}
						} else if p.mode == rankMax {
							r = maxf32(r, px.r)
							g = maxf32(g, px.g)
							b = maxf32(b, px.b)
							if !opaque {
								a = maxf32(a, px.a)
							}
						}
						sz++
					}
				}

				if p.mode == rankMedian {
					sort(rbuf)
					sort(gbuf)
					sort(bbuf)
					if !opaque {
						sort(abuf)
					}

					idx := sz / 2
					r, g, b = rbuf[idx], gbuf[idx], bbuf[idx]
					if !opaque {
						a = abuf[idx]
					}
				}

				if opaque {
					a = 1
				}

				pixSetter.setPixel(dstb.Min.X+x-srcb.Min.X, dstb.Min.Y+y-srcb.Min.Y, pixel{r, g, b, a})

				// rotate buffer columns
				if x < srcb.Max.X-1 {
					copy(pxbuf[0:], pxbuf[ksize:])
					pxbuf = pxbuf[0 : ksize*(ksize-1)]
					kx := x + 1 + kradius
					if kx > srcb.Max.X-1 {
						kx = srcb.Max.X - 1
					}
					for j := y - kradius; j <= y+kradius; j++ {
						ky := j
						if ky < srcb.Min.Y {
							ky = srcb.Min.Y
						} else if ky > srcb.Max.Y-1 {
							ky = srcb.Max.Y - 1
						}
						pxbuf = append(pxbuf, pixGetter.getPixel(kx, ky))
					}
				}
			}
		}
	})
}

// Median creates a median image filter.
// Picks a median value per channel in neighborhood for each pixel.
// The ksize parameter is the kernel size. It must be an odd positive integer (for example: 3, 5, 7).
// If the disk parameter is true, a disk-shaped neighborhood will be used instead of a square neighborhood.
func Median(ksize int, disk bool) Filter {
	return &rankFilter{
		ksize: ksize,
		disk:  disk,
		mode:  rankMedian,
	}
}

// Minimum creates a local minimum image filter.
// Picks a minimum value per channel in neighborhood for each pixel.
// The ksize parameter is the kernel size. It must be an odd positive integer (for example: 3, 5, 7).
// If the disk parameter is true, a disk-shaped neighborhood will be used instead of a square neighborhood.
func Minimum(ksize int, disk bool) Filter {
	return &rankFilter{
		ksize: ksize,
		disk:  disk,
		mode:  rankMin,
	}
}

// Maximum creates a local maximum image filter.
// Picks a maximum value per channel in neighborhood for each pixel.
// The ksize parameter is the kernel size. It must be an odd positive integer (for example: 3, 5, 7).
// If the disk parameter is true, a disk-shaped neighborhood will be used instead of a square neighborhood.
func Maximum(ksize int, disk bool) Filter {
	return &rankFilter{
		ksize: ksize,
		disk:  disk,
		mode:  rankMax,
	}
}
