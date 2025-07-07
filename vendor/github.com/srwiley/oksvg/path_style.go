// Copyright 2017 The oksvg Authors. All rights reserved.
// created: 2/12/2017 by S.R.Wiley
//
// utils.go implements translation of an SVG2.0 path into a rasterx Path.

package oksvg

import (
	"image/color"

	"github.com/srwiley/rasterx"
)

// PathStyle holds the state of the SVG style.
type PathStyle struct {
	FillOpacity, LineOpacity          float64
	LineWidth, DashOffset, MiterLimit float64
	Dash                              []float64
	UseNonZeroWinding                 bool
	fillerColor, linerColor           interface{} // either color.Color or rasterx.Gradient
	LineGap                           rasterx.GapFunc
	LeadLineCap                       rasterx.CapFunc // This is used if different than LineCap
	LineCap                           rasterx.CapFunc
	LineJoin                          rasterx.JoinMode
	mAdder                            rasterx.MatrixAdder // current transform
}

// styleAttribute describes draw options, such as {"fill":"black"; "stroke":"white"}.
type styleAttribute = map[string]string

// DefaultStyle sets the default PathStyle to fill black, winding rule,
// full opacity, no stroke, ButtCap line end and Bevel line connect.
var DefaultStyle = PathStyle{1.0, 1.0, 2.0, 0.0, 4.0, nil, true,
	color.NRGBA{0x00, 0x00, 0x00, 0xff}, nil,
	nil, nil, rasterx.ButtCap, rasterx.Bevel, rasterx.MatrixAdder{M: rasterx.Identity}}
