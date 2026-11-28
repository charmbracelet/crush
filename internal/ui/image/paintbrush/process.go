package paintbrush

import (
	"fmt"
	"math"
)

type Task struct {
	CharX, CharY int
}

type TaskResult struct {
	CharX, CharY int
	Fg, Bg       Vec4
	Glyph        *Glyph
}

func (c *Canvas) processTask(task Task, imgCharWidth, imgCharHeight float64) TaskResult {
	imgXBegin := float64(task.CharX) * imgCharWidth
	imgYBegin := float64(task.CharY) * imgCharHeight

	bestGlyph := c.Font.Glyphs[' ']
	bestErr := math.MaxFloat64
	var bestFg, bestBg Vec4

	for _, glyph := range c.Font.Glyphs {
		if c.IsForbiddenCharacter(rune(glyph.Unicode)) {
			continue
		}

		fgSum := 0.0
		fgCol := Vec4{}
		bgSum := 0.0
		bgCol := Vec4{}

		for fontCharX := 0; fontCharX < c.Font.GlyphWidth; fontCharX++ {
			for fontCharY := 0; fontCharY < c.Font.GlyphHeight; fontCharY++ {
				index := fontCharX + fontCharY*c.Font.GlyphWidth
				if index >= len(glyph.Pixels) {
					fmt.Printf("Warning: Index out of range for glyph '%s'. Index: %d, Pixel array length: %d\n", glyph.UTF8, index, len(glyph.Pixels))
					continue
				}
				fg := float64(glyph.Pixels[index]) / 255.0
				bg := 1.0 - fg
				fgSum += fg
				bgSum += bg

				imgX := imgXBegin + imgCharWidth*(float64(fontCharX)+0.5)/float64(c.Font.GlyphWidth)
				imgY := imgYBegin + imgCharHeight*(float64(fontCharY)+0.5)/float64(c.Font.GlyphHeight)
				col := c.readImageColor(imgX, imgY)

				fgCol = fgCol.Add(col.Mul(fg))
				bgCol = bgCol.Add(col.Mul(bg))
			}
		}

		if fgSum > 0 {
			fgCol = fgCol.Div(fgSum)
		}
		fgCol.A = 1

		if bgSum > 0 {
			bgCol = bgCol.Div(bgSum)
		}
		if bgCol.A < 0.2 {
			bgCol.A = 0
		} else {
			bgCol.A = 1
		}
		bgCol = bgCol.Mul(bgCol.A) // premultiply

		error := c.calculateError(&glyph, fgCol, bgCol, imgXBegin, imgYBegin, imgCharWidth, imgCharHeight)
		error /= glyph.Weight

		if error < bestErr {
			bestErr = error
			bestGlyph = glyph
			bestFg = fgCol
			bestBg = bgCol
		}
	}

	// Blit the character onto resultRGBABytes
	c.blitCharacter(task.CharX, task.CharY, &bestGlyph, bestFg, bestBg)

	return TaskResult{
		CharX: task.CharX,
		CharY: task.CharY,
		Fg:    bestFg,
		Bg:    bestBg,
		Glyph: &bestGlyph,
	}
}

func (c *Canvas) calculateError(glyph *Glyph, fgCol, bgCol Vec4, imgXBegin, imgYBegin, imgCharWidth, imgCharHeight float64) float64 {
	error := 0.0
	for fontCharX := 0; fontCharX < c.Font.GlyphWidth; fontCharX++ {
		for fontCharY := 0; fontCharY < c.Font.GlyphHeight; fontCharY++ {
			fg := float64(glyph.Pixels[fontCharX+fontCharY*c.Font.GlyphWidth]) / 255.0
			bg := 1.0 - fg
			imgX := imgXBegin + imgCharWidth*(float64(fontCharX)+0.5)/float64(c.Font.GlyphWidth)
			imgY := imgYBegin + imgCharHeight*(float64(fontCharY)+0.5)/float64(c.Font.GlyphHeight)
			col := c.readImageColor(imgX, imgY)
			col = col.Mul(col.A) // premultiply
			x := fgCol.Mul(fg).Add(bgCol.Mul(bg))
			d := col.Sub(x)
			error += d.Dot(d)
		}
	}

	if weight, exists := c.Weights[rune(glyph.Unicode)]; exists {
		error /= weight
	}

	return error
}

func (c *Canvas) blitCharacter(charX, charY int, glyph *Glyph, fg, bg Vec4) {
	for fontCharX := 0; fontCharX < c.Font.GlyphWidth; fontCharX++ {
		for fontCharY := 0; fontCharY < c.Font.GlyphHeight; fontCharY++ {
			resultX := charX*c.Font.GlyphWidth + fontCharX
			resultY := charY*c.Font.GlyphHeight + fontCharY
			idx := (resultY*c.ResultRGBAWidth + resultX) * 4
			fgFactor := float64(glyph.Pixels[fontCharX+fontCharY*c.Font.GlyphWidth]) / 255.0
			bgFactor := 1.0 - fgFactor
			pixel := fg.Mul(fgFactor).Add(bg.Mul(bgFactor))
			c.ResultRGBABytes[idx] = uint8(pixel.R * 255)
			c.ResultRGBABytes[idx+1] = uint8(pixel.G * 255)
			c.ResultRGBABytes[idx+2] = uint8(pixel.B * 255)
			c.ResultRGBABytes[idx+3] = uint8(pixel.A * 255)
		}
	}
}
