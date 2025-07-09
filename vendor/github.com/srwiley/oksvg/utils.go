// Copyright 2017 The oksvg Authors. All rights reserved.
// created: 2/12/2017 by S.R.Wiley
//
// utils.go implements translation of an SVG2.0 path into a rasterx Path.

package oksvg

import (
	"errors"
	"image/color"
	"strconv"
	"strings"

	"github.com/srwiley/rasterx"
	"golang.org/x/image/colornames"
)

// unitSuffixes are suffixes sometimes applied to the width and height attributes
// of the svg element.
var unitSuffixes = []string{"cm", "mm", "px", "pt"}

func parseColorValue(v string) (uint8, error) {
	if v[len(v)-1] == '%' {
		n, err := strconv.Atoi(strings.TrimSpace(v[:len(v)-1]))
		if err != nil {
			return 0, err
		}
		return uint8(n * 0xFF / 100), nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if n > 255 {
		n = 255
	}
	return uint8(n), err
}

// trimSuffixes removes unitSuffixes from any number that is not just numeric
func trimSuffixes(a string) (b string) {
	if a == "" || (a[len(a)-1] >= '0' && a[len(a)-1] <= '9') {
		return a
	}
	b = a
	for _, v := range unitSuffixes {
		b = strings.TrimSuffix(b, v)
	}
	return
}

// parseFloat is a helper function that strips suffixes before passing to strconv.ParseFloat
func parseFloat(s string, bitSize int) (float64, error) {
	val := trimSuffixes(s)
	return strconv.ParseFloat(val, bitSize)
}

// splitOnCommaOrSpace returns a list of strings after splitting the input on comma and space delimiters
func splitOnCommaOrSpace(s string) []string {
	return strings.FieldsFunc(s,
		func(r rune) bool {
			return r == ',' || r == ' '
		})
}

func parseClasses(data string) (map[string]styleAttribute, error) {
	res := map[string]styleAttribute{}
	arr := strings.Split(data, "}")
	for _, v := range arr {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		valueIndex := strings.Index(v, "{")
		if valueIndex == -1 || valueIndex == len(v)-1 {
			return res, errors.New(v + "}: invalid map format in class definitions")
		}
		classesStr := v[:valueIndex]
		attrStr := v[valueIndex+1:]
		attrMap, err := parseAttrs(attrStr)
		if err != nil {
			return res, err
		}
		classes := strings.Split(classesStr, ",")
		for _, class := range classes {
			class = strings.TrimSpace(class)
			if len(class) > 0 && class[0] == '.' {
				class = class[1:]
			}
			for attrKey, attrVal := range attrMap {
				if res[class] == nil {
					res[class] = make(styleAttribute, len(attrMap))
				}
				res[class][attrKey] = attrVal
			}
		}
	}
	return res, nil
}

func parseAttrs(attrStr string) (styleAttribute, error) {
	arr := strings.Split(attrStr, ";")
	res := make(styleAttribute, len(arr))
	for _, kv := range arr {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		tmp := strings.SplitN(kv, ":", 2)
		if len(tmp) != 2 {
			return res, errors.New(kv + ": invalid attribute format")
		}
		k := strings.TrimSpace(tmp[0])
		v := strings.TrimSpace(tmp[1])
		res[k] = v
	}
	return res, nil
}

func readFraction(v string) (f float64, err error) {
	v = strings.TrimSpace(v)
	d := 1.0
	if strings.HasSuffix(v, "%") {
		d = 100
		v = strings.TrimSuffix(v, "%")
	}
	f, err = parseFloat(v, 64)
	f /= d
	// Is this is an unnecessary restriction? For now fractions can be all values not just in the range [0,1]
	// if f > 1 {
	// 	f = 1
	// } else if f < 0 {
	// 	f = 0
	// }
	return
}

// getColor is a helper function to get the background color
// if ReadGradUrl needs it.
func getColor(clr interface{}) color.Color {
	switch c := clr.(type) {
	case rasterx.Gradient: // This is a bit lazy but oh well
		for _, s := range c.Stops {
			if s.StopColor != nil {
				return s.StopColor
			}
		}
	case color.NRGBA:
		return c
	}
	return colornames.Black
}

func localizeGradIfStopClrNil(g *rasterx.Gradient, defaultColor interface{}) (grad rasterx.Gradient) {
	grad = *g
	for _, s := range grad.Stops {
		if s.StopColor == nil { // This means we need copy the gradient's Stop slice
			// and fill in the default color

			// Copy the stops
			stops := make([]rasterx.GradStop, len(grad.Stops))
			copy(stops, grad.Stops)
			grad.Stops = stops
			// Use the background color when a stop color is nil
			clr := getColor(defaultColor)
			for i, s := range stops {
				if s.StopColor == nil {
					grad.Stops[i].StopColor = clr
				}
			}
			break // Only need to do this once
		}
	}
	return
}
