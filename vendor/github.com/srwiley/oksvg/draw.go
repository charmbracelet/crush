// Copyright 2017 The oksvg Authors. All rights reserved.
// created: 2/12/2017 by S.R.Wiley
//
// utils.go implements translation of an SVG2.0 path into a rasterx Path.

package oksvg

import (
	"encoding/xml"
	"errors"
	"log"
	"strings"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/math/fixed"
)

// svgFunc defines function interface to use as drawing implementation.
type svgFunc func(c *IconCursor, attrs []xml.Attr) error

var (
	drawFuncs = map[string]svgFunc{
		"svg":            svgF,
		"g":              gF,
		"line":           lineF,
		"stop":           stopF,
		"rect":           rectF,
		"circle":         circleF,
		"ellipse":        circleF, //circleF handles ellipse also
		"polyline":       polylineF,
		"polygon":        polygonF,
		"path":           pathF,
		"desc":           descF,
		"defs":           defsF,
		"style":          styleF,
		"title":          titleF,
		"linearGradient": linearGradientF,
		"radialGradient": radialGradientF,
	}

	svgF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		c.icon.ViewBox.X = 0
		c.icon.ViewBox.Y = 0
		c.icon.ViewBox.W = 0
		c.icon.ViewBox.H = 0
		var width, height float64
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "viewBox":
				err = c.GetPoints(attr.Value)
				if len(c.points) != 4 {
					return errParamMismatch
				}
				c.icon.ViewBox.X = c.points[0]
				c.icon.ViewBox.Y = c.points[1]
				c.icon.ViewBox.W = c.points[2]
				c.icon.ViewBox.H = c.points[3]
			case "width":
				width, err = parseFloat(attr.Value, 64)
			case "height":
				height, err = parseFloat(attr.Value, 64)
			}
			if err != nil {
				return err
			}
		}
		if c.icon.ViewBox.W == 0 {
			c.icon.ViewBox.W = width
		}
		if c.icon.ViewBox.H == 0 {
			c.icon.ViewBox.H = height
		}
		return nil
	}
	gF    svgFunc = func(*IconCursor, []xml.Attr) error { return nil } // g does nothing but push the style
	rectF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var x, y, w, h, rx, ry float64
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "x":
				x, err = parseFloat(attr.Value, 64)
			case "y":
				y, err = parseFloat(attr.Value, 64)
			case "width":
				w, err = parseFloat(attr.Value, 64)
			case "height":
				h, err = parseFloat(attr.Value, 64)
			case "rx":
				rx, err = parseFloat(attr.Value, 64)
			case "ry":
				ry, err = parseFloat(attr.Value, 64)
			}
			if err != nil {
				return err
			}
		}
		if w == 0 || h == 0 {
			return nil
		}
		rasterx.AddRoundRect(x, y, w+x, h+y, rx, ry, 0, rasterx.RoundGap, &c.Path)
		return nil
	}
	circleF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var cx, cy, rx, ry float64
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "cx":
				cx, err = parseFloat(attr.Value, 64)
			case "cy":
				cy, err = parseFloat(attr.Value, 64)
			case "r":
				rx, err = parseFloat(attr.Value, 64)
				ry = rx
			case "rx":
				rx, err = parseFloat(attr.Value, 64)
			case "ry":
				ry, err = parseFloat(attr.Value, 64)
			}
			if err != nil {
				return err
			}
		}
		if rx == 0 || ry == 0 { // not drawn, but not an error
			return nil
		}
		c.EllipseAt(cx, cy, rx, ry)
		return nil
	}
	lineF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var x1, x2, y1, y2 float64
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "x1":
				x1, err = parseFloat(attr.Value, 64)
			case "x2":
				x2, err = parseFloat(attr.Value, 64)
			case "y1":
				y1, err = parseFloat(attr.Value, 64)
			case "y2":
				y2, err = parseFloat(attr.Value, 64)
			}
			if err != nil {
				return err
			}
		}
		c.Path.Start(fixed.Point26_6{
			X: fixed.Int26_6((x1) * 64),
			Y: fixed.Int26_6((y1) * 64)})
		c.Path.Line(fixed.Point26_6{
			X: fixed.Int26_6((x2) * 64),
			Y: fixed.Int26_6((y2) * 64)})
		return nil
	}
	polylineF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "points":
				err = c.GetPoints(attr.Value)
				if len(c.points)%2 != 0 {
					return errors.New("polygon has odd number of points")
				}
			}
			if err != nil {
				return err
			}
		}
		if len(c.points) > 4 {
			c.Path.Start(fixed.Point26_6{
				X: fixed.Int26_6((c.points[0]) * 64),
				Y: fixed.Int26_6((c.points[1]) * 64)})
			for i := 2; i < len(c.points)-1; i += 2 {
				c.Path.Line(fixed.Point26_6{
					X: fixed.Int26_6((c.points[i]) * 64),
					Y: fixed.Int26_6((c.points[i+1]) * 64)})
			}
		}
		return nil
	}
	polygonF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		err := polylineF(c, attrs)
		if len(c.points) > 4 {
			c.Path.Stop(true)
		}
		return err
	}
	pathF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "d":
				err = c.CompilePath(attr.Value)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
	descF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		c.inDescText = true
		c.icon.Descriptions = append(c.icon.Descriptions, "")
		return nil
	}
	titleF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		c.inTitleText = true
		c.icon.Titles = append(c.icon.Titles, "")
		return nil
	}
	defsF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		c.inDefs = true
		return nil
	}
	styleF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		c.inDefsStyle = true
		return nil
	}
	linearGradientF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var err error
		c.inGrad = true
		c.grad = &rasterx.Gradient{Points: [5]float64{0, 0, 1, 0, 0},
			IsRadial: false, Bounds: c.icon.ViewBox, Matrix: rasterx.Identity}
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "id":
				id := attr.Value
				if len(id) >= 0 {
					c.icon.Grads[id] = c.grad
				} else {
					return errZeroLengthID
				}
			case "x1":
				c.grad.Points[0], err = readFraction(attr.Value)
			case "y1":
				c.grad.Points[1], err = readFraction(attr.Value)
			case "x2":
				c.grad.Points[2], err = readFraction(attr.Value)
			case "y2":
				c.grad.Points[3], err = readFraction(attr.Value)
			default:
				err = c.ReadGradAttr(attr)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
	radialGradientF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		c.inGrad = true
		c.grad = &rasterx.Gradient{Points: [5]float64{0.5, 0.5, 0.5, 0.5, 0.5},
			IsRadial: true, Bounds: c.icon.ViewBox, Matrix: rasterx.Identity}
		var setFx, setFy bool
		var err error
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "id":
				id := attr.Value
				if len(id) >= 0 {
					c.icon.Grads[id] = c.grad
				} else {
					return errZeroLengthID
				}
			case "r":
				c.grad.Points[4], err = readFraction(attr.Value)
			case "cx":
				c.grad.Points[0], err = readFraction(attr.Value)
			case "cy":
				c.grad.Points[1], err = readFraction(attr.Value)
			case "fx":
				setFx = true
				c.grad.Points[2], err = readFraction(attr.Value)
			case "fy":
				setFy = true
				c.grad.Points[3], err = readFraction(attr.Value)
			default:
				err = c.ReadGradAttr(attr)
			}
			if err != nil {
				return err
			}
		}
		if !setFx { // set fx to cx by default
			c.grad.Points[2] = c.grad.Points[0]
		}
		if !setFy { // set fy to cy by default
			c.grad.Points[3] = c.grad.Points[1]
		}
		return nil
	}
	stopF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var err error
		if c.inGrad {
			stop := rasterx.GradStop{Opacity: 1.0}
			for _, attr := range attrs {
				switch attr.Name.Local {
				case "offset":
					stop.Offset, err = readFraction(attr.Value)
				case "stop-color":
					//todo: add current color inherit
					stop.StopColor, err = ParseSVGColor(attr.Value)
				case "stop-opacity":
					stop.Opacity, err = parseFloat(attr.Value, 64)
				}
				if err != nil {
					return err
				}
			}
			c.grad.Stops = append(c.grad.Stops, stop)
		}
		return nil
	}
	useF svgFunc = func(c *IconCursor, attrs []xml.Attr) error {
		var (
			href string
			x, y float64
			err  error
		)
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "href":
				href = attr.Value
			case "x":
				x, err = parseFloat(attr.Value, 64)
			case "y":
				y, err = parseFloat(attr.Value, 64)
			}
			if err != nil {
				return err
			}
		}
		// Translate the Style adder matrix by use's x and y
		c.StyleStack[len(c.StyleStack)-1].mAdder.M =
			c.StyleStack[len(c.StyleStack)-1].mAdder.M.Translate(x, y)
		if href == "" {
			return errors.New("only use tags with href is supported")
		}
		if !strings.HasPrefix(href, "#") {
			return errors.New("only the ID CSS selector is supported")
		}
		defs, ok := c.icon.Defs[href[1:]]
		if !ok {
			return errors.New("href ID in use statement was not found in saved defs")
		}
		for _, def := range defs {
			if def.Tag == "endg" {
				// pop style
				c.StyleStack = c.StyleStack[:len(c.StyleStack)-1]
				continue
			}
			if err = c.PushStyle(def.Attrs); err != nil {
				return err
			}
			df, ok := drawFuncs[def.Tag]
			if !ok {
				errStr := "Cannot process svg element " + def.Tag
				if c.ErrorMode == StrictErrorMode {
					return errors.New(errStr)
				} else if c.ErrorMode == WarnErrorMode {
					log.Println(errStr)
				}
				return nil
			}
			if err := df(c, def.Attrs); err != nil {
				return err
			}
			//Did c.Path get added to during the drawFunction call iteration?
			if len(c.Path) > 0 {
				//The cursor parsed a path from the xml element
				pathCopy := make(rasterx.Path, len(c.Path))
				copy(pathCopy, c.Path)
				c.icon.SVGPaths = append(c.icon.SVGPaths, SvgPath{c.StyleStack[len(c.StyleStack)-1], pathCopy})
				c.Path = c.Path[:0]
			}
			if def.Tag != "g" {
				// pop style
				c.StyleStack = c.StyleStack[:len(c.StyleStack)-1]
			}
		}
		return nil
	}
)

func init() {
	// avoids cyclical static declaration
	// called on package initialization
	drawFuncs["use"] = useF
}
