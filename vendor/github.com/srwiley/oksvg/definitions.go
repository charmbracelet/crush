// Copyright 2017 The oksvg Authors. All rights reserved.
// created: 2/12/2017 by S.R.Wiley
//
// utils.go implements translation of an SVG2.0 path into a rasterx Path.

package oksvg

import (
	"encoding/xml"
)

// definition is used to store XML-tags of SVG source definitions data.
type definition struct {
	ID, Tag string
	Attrs   []xml.Attr
}
