// Copyright 2017 The oksvg Authors. All rights reserved.
// created: 2/12/2017 by S.R.Wiley
//
// utils.go implements translation of an SVG2.0 path into a rasterx Path.

package oksvg

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image/color"
	"io"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/colornames"
	"golang.org/x/net/html/charset"
)

// ReadIconStream reads the Icon from the given io.Reader.
// This only supports a sub-set of SVG, but
// is enough to draw many icons. If errMode is provided,
// the first value determines if the icon ignores, errors out, or logs a warning
// if it does not handle an element found in the icon file. Ignore warnings is
// the default if no ErrorMode value is provided.
func ReadIconStream(stream io.Reader, errMode ...ErrorMode) (*SvgIcon, error) {
	icon := &SvgIcon{Defs: make(map[string][]definition), Grads: make(map[string]*rasterx.Gradient), Transform: rasterx.Identity}
	cursor := &IconCursor{StyleStack: []PathStyle{DefaultStyle}, icon: icon}
	if len(errMode) > 0 {
		cursor.ErrorMode = errMode[0]
	}
	classInfo := ""
	decoder := xml.NewDecoder(stream)
	decoder.CharsetReader = charset.NewReaderLabel
	for {
		t, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return icon, err
		}
		// Inspect the type of the XML token
		switch se := t.(type) {
		case xml.StartElement:
			// Reads all recognized style attributes from the start element
			// and places it on top of the styleStack
			err = cursor.PushStyle(se.Attr)
			if err != nil {
				return icon, err
			}
			err = cursor.readStartElement(se)
			if err != nil {
				return icon, err
			}
			if se.Name.Local == "style" && cursor.inDefs {
				cursor.inDefsStyle = true
			}
		case xml.EndElement:
			// pop style
			cursor.StyleStack = cursor.StyleStack[:len(cursor.StyleStack)-1]
			switch se.Name.Local {
			case "g":
				if cursor.inDefs {
					cursor.currentDef = append(cursor.currentDef, definition{
						Tag: "endg",
					})
				}
			case "title":
				cursor.inTitleText = false
			case "desc":
				cursor.inDescText = false
			case "defs":
				if len(cursor.currentDef) > 0 {
					cursor.icon.Defs[cursor.currentDef[0].ID] = cursor.currentDef
					cursor.currentDef = make([]definition, 0)
				}
				cursor.inDefs = false
			case "radialGradient", "linearGradient":
				cursor.inGrad = false

			case "style":
				if cursor.inDefsStyle {
					icon.classes, err = parseClasses(classInfo)
					if err != nil {
						return icon, err
					}
					cursor.inDefsStyle = false
				}
			}
		case xml.CharData:
			if cursor.inTitleText {
				icon.Titles[len(icon.Titles)-1] += string(se)
			}
			if cursor.inDescText {
				icon.Descriptions[len(icon.Descriptions)-1] += string(se)
			}
			if cursor.inDefsStyle {
				classInfo = string(se)
			}
		}
	}
	return icon, nil
}

// ReadReplacingCurrentColor replaces currentColor value with specified value and loads SvgIcon as ReadIconStream do.
// currentColor value should be valid hex, rgb or named color value.
func ReadReplacingCurrentColor(stream io.Reader, currentColor string, errMode ...ErrorMode) (icon *SvgIcon, err error) {
	var (
		data []byte
	)

	if data, err = ioutil.ReadAll(stream); err != nil {
		return nil, fmt.Errorf("%w: read data: %v", errParamMismatch, err)
	}

	if currentColor != "" && strings.Contains(string(data), "currentColor") {
		data = []byte(strings.ReplaceAll(string(data), "currentColor", currentColor))
	}

	if icon, err = ReadIconStream(bytes.NewBuffer(data), errMode...); err != nil {
		return nil, fmt.Errorf("%w: load: %v", errParamMismatch, err)
	}

	return icon, nil
}

// ReadIcon reads the Icon from the named file.
// This only supports a sub-set of SVG, but is enough to draw many icons.
// If errMode is provided, the first value determines if the icon ignores, errors out, or logs a warning
// if it does not handle an element found in the icon file.
// Ignore warnings is the default if no ErrorMode value is provided.
func ReadIcon(iconFile string, errMode ...ErrorMode) (*SvgIcon, error) {
	fin, errf := os.Open(iconFile)
	if errf != nil {
		return nil, errf
	}
	defer fin.Close()
	return ReadIconStream(fin, errMode...)
}

// ParseSVGColorNum reads the SFG color string e.g. #FBD9BD
func ParseSVGColorNum(colorStr string) (r, g, b uint8, err error) {
	colorStr = strings.TrimPrefix(colorStr, "#")
	var t uint64
	if len(colorStr) != 6 {
		if len(colorStr) != 3 {
			err = fmt.Errorf("color string %s is not length 3 or 6 as required by SVG specification",
				colorStr)
			return
		}
		// SVG specs say duplicate characters in case of 3 digit hex number
		colorStr = string([]byte{colorStr[0], colorStr[0],
			colorStr[1], colorStr[1], colorStr[2], colorStr[2]})
	}
	for _, v := range []struct {
		c *uint8
		s string
	}{
		{&r, colorStr[0:2]},
		{&g, colorStr[2:4]},
		{&b, colorStr[4:6]}} {
		t, err = strconv.ParseUint(v.s, 16, 8)
		if err != nil {
			return
		}
		*v.c = uint8(t)
	}
	return
}

// ParseSVGColor parses an SVG color string in all forms
// including all SVG1.1 names, obtained from the image.colornames package
func ParseSVGColor(colorStr string) (color.Color, error) {
	// _, _, _, a := curColor.RGBA()
	v := strings.ToLower(colorStr)
	if strings.HasPrefix(v, "url") { // We are not handling urls
		// and gradients and stuff at this point
		return color.NRGBA{0, 0, 0, 255}, nil
	}
	switch v {
	case "none", "":
		// nil signals that the function (fill or stroke) is off;
		// not the same as black
		return nil, nil
	default:
		cn, ok := colornames.Map[v]
		if ok {
			r, g, b, a := cn.RGBA()
			return color.NRGBA{uint8(r), uint8(g), uint8(b), uint8(a)}, nil
		}
	}
	cStr := strings.TrimPrefix(colorStr, "rgb(")
	if cStr != colorStr {
		cStr := strings.TrimSuffix(cStr, ")")
		vals := strings.Split(cStr, ",")
		if len(vals) != 3 {
			return color.NRGBA{}, errParamMismatch
		}
		var cvals [3]uint8
		var err error
		for i := range cvals {
			cvals[i], err = parseColorValue(vals[i])
			if err != nil {
				return nil, err
			}
		}
		return color.NRGBA{cvals[0], cvals[1], cvals[2], 0xFF}, nil
	}

	cStr = strings.TrimPrefix(colorStr, "hsl(")
	if cStr != colorStr {
		cStr := strings.TrimSuffix(cStr, ")")
		vals := strings.Split(cStr, ",")
		if len(vals) != 3 {
			return color.NRGBA{}, errParamMismatch
		}

		H, err := strconv.ParseInt(strings.TrimSpace(vals[0]), 10, 64)
		if err != nil {
			return color.NRGBA{}, fmt.Errorf("invalid hue in hsl: '%s' (%s)", vals[0], err)
		}

		S, err := strconv.ParseFloat(strings.TrimSpace(vals[1][:len(vals[1])-1]), 64)
		if err != nil {
			return color.NRGBA{}, fmt.Errorf("invalid saturation in hsl: '%s' (%s)", vals[1], err)
		}
		S = S / 100

		L, err := strconv.ParseFloat(strings.TrimSpace(vals[2][:len(vals[2])-1]), 64)
		if err != nil {
			return color.NRGBA{}, fmt.Errorf("invalid lightness in hsl: '%s' (%s)", vals[2], err)
		}
		L = L / 100

		C := (1 - math.Abs((2*L)-1)) * S
		X := C * (1 - math.Abs(math.Mod((float64(H)/60), 2)-1))
		m := L - C/2

		var rp, gp, bp float64
		if H < 60 {
			rp, gp, bp = float64(C), float64(X), float64(0)
		} else if H < 120 {
			rp, gp, bp = float64(X), float64(C), float64(0)
		} else if H < 180 {
			rp, gp, bp = float64(0), float64(C), float64(X)
		} else if H < 240 {
			rp, gp, bp = float64(0), float64(X), float64(C)
		} else if H < 300 {
			rp, gp, bp = float64(X), float64(0), float64(C)
		} else {
			rp, gp, bp = float64(C), float64(0), float64(X)
		}

		r, g, b := math.Round((rp+m)*255), math.Round((gp+m)*255), math.Round((bp+m)*255)
		if r > 255 {
			r = 255
		}
		if g > 255 {
			g = 255
		}
		if b > 255 {
			b = 255
		}

		return color.NRGBA{
			uint8(r),
			uint8(g),
			uint8(b),
			0xFF,
		}, nil
	}

	if colorStr[0] == '#' {
		r, g, b, err := ParseSVGColorNum(colorStr)
		if err != nil {
			return nil, err
		}
		return color.NRGBA{r, g, b, 0xFF}, nil
	}
	return nil, errParamMismatch
}
