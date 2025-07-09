// Package charmtone contains an API for the CharmTone color palette.
package charmtone

import (
	"fmt"
	"image/color"
	"slices"

	"github.com/lucasb-eyer/go-colorful"
)

var _ color.Color = Key(0)

// Key is a type for color keys.
type Key int

// Available colors.
const (
	Cumin Key = iota
	Tang
	Yam
	Paprika
	Bengal
	Uni
	Sriracha
	Coral
	Salmon
	Chili
	Cherry
	Tuna
	Macaron
	Pony
	Cheeky
	Flamingo
	Dolly
	Blush
	Urchin
	Mochi
	Lilac
	Prince
	Violet
	Mauve
	Grape
	Plum
	Orchid
	Jelly
	Charple
	Hazy
	Ox
	Sapphire
	Guppy
	Oceania
	Thunder
	Anchovy
	Damson
	Malibu
	Sardine
	Zinc
	Turtle
	Lichen
	Guac
	Julep
	Bok
	Mustard
	Citron
	Zest
	Pepper
	Barbeque
	Charcoal
	Iron
	Oyster
	Squid
	Smoke
	Ash
	Salt
	Butter
)

// RGBA returns the red, green, blue, and alpha values of the color. It
// satisfies the color.Color interface.
func (k Key) RGBA() (r, g, b, a uint32) {
	c, err := colorful.Hex(k.Hex())
	if err != nil {
		panic(fmt.Sprintf("invalid color key %d: %s: %v", k, k.String(), err))
	}
	return c.RGBA()
}

// String returns the official CharmTone name of the color. It satisfies the
// fmt.Stringer interface.
func (k Key) String() string {
	return map[Key]string{
		Cumin:    "Cumin",
		Tang:     "Tang",
		Yam:      "Yam",
		Paprika:  "Paprika",
		Bengal:   "Bengal",
		Uni:      "Uni",
		Sriracha: "Sriracha",
		Coral:    "Coral",
		Salmon:   "Salmon",
		Chili:    "Chili",
		Cherry:   "Cherry",
		Tuna:     "Tuna",
		Macaron:  "Macaron",
		Pony:     "Pony",
		Cheeky:   "Cheeky",
		Flamingo: "Flamingo",
		Dolly:    "Dolly",
		Blush:    "Blush",
		Urchin:   "Urchin",
		Mochi:    "Crystal",
		Lilac:    "Lilac",
		Prince:   "Prince",
		Violet:   "Violet",
		Mauve:    "Mauve",
		Grape:    "Grape",
		Plum:     "Plum",
		Orchid:   "Orchid",
		Jelly:    "Jelly",
		Charple:  "Charple",
		Hazy:     "Hazy",
		Ox:       "Ox",
		Sapphire: "Sapphire",
		Guppy:    "Guppy",
		Oceania:  "Oceania",
		Thunder:  "Thunder",
		Anchovy:  "Anchovy",
		Damson:   "Damson",
		Malibu:   "Malibu",
		Sardine:  "Sardine",
		Zinc:     "Zinc",
		Turtle:   "Turtle",
		Lichen:   "Lichen",
		Guac:     "Guac",
		Julep:    "Julep",
		Bok:      "Bok",
		Mustard:  "Mustard",
		Citron:   "Citron",
		Zest:     "Zest",
		Pepper:   "Pepper",
		Barbeque: "Barbeque",
		Charcoal: "Charcoal",
		Iron:     "Iron",
		Oyster:   "Oyster",
		Squid:    "Squid",
		Smoke:    "Smoke",
		Salt:     "Salt",
		Ash:      "Ash",
		Butter:   "Butter",
	}[k]
}

// Hex returns the hex value of the color.
func (k Key) Hex() string {
	return map[Key]string{
		Cumin:    "#BF976F",
		Tang:     "#FF985A",
		Yam:      "#FFB587",
		Paprika:  "#D36C64",
		Bengal:   "#FF6E63",
		Uni:      "#FF937D",
		Sriracha: "#EB4268",
		Coral:    "#FF577D",
		Salmon:   "#FF7F90",
		Chili:    "#E23080",
		Cherry:   "#FF388B",
		Tuna:     "#FF6DAA",
		Macaron:  "#E940B0",
		Pony:     "#FF4FBF",
		Cheeky:   "#FF79D0",
		Flamingo: "#F947E3",
		Dolly:    "#FF60FF",
		Blush:    "#FF84FF",
		Urchin:   "#C337E0",
		Mochi:    "#EB5DFF",
		Lilac:    "#F379FF",
		Prince:   "#9C35E1",
		Violet:   "#C259FF",
		Mauve:    "#D46EFF",
		Grape:    "#7134DD",
		Plum:     "#9953FF",
		Orchid:   "#AD6EFF",
		Jelly:    "#4A30D9",
		Charple:  "#6B50FF",
		Hazy:     "#8B75FF",
		Ox:       "#3331B2",
		Sapphire: "#4949FF",
		Guppy:    "#7272FF",
		Oceania:  "#2B55B3",
		Thunder:  "#4776FF",
		Anchovy:  "#719AFC",
		Damson:   "#007AB8",
		Malibu:   "#00A4FF",
		Sardine:  "#4FBEFE",
		Zinc:     "#0e9996",
		Turtle:   "#0ADCD9",
		Lichen:   "#5CDFEA",
		Guac:     "#00b875",
		Julep:    "#00FFB2",
		Bok:      "#68FFD6",
		Mustard:  "#F5EF34",
		Citron:   "#E8FF27",
		Zest:     "#E8FE96",
		Pepper:   "#201F26",
		Barbeque: "#2d2c35",
		Charcoal: "#3A3943",
		Iron:     "#4D4C57",
		Oyster:   "#605F6B",
		Squid:    "#858392",
		Smoke:    "#BFBCC8",
		Ash:      "#DFDBDD",
		Salt:     "#F1EFEF",
		Butter:   "#FFFAF1",
	}[k]
}

// Keys returns a slice of all CharmTone color keys.
func Keys() []Key {
	return []Key{
		Cumin,
		Tang,
		Yam,
		Paprika,
		Bengal,
		Uni,
		Sriracha,
		Coral,
		Salmon,
		Chili,
		Cherry,
		Tuna,
		Macaron,
		Pony,
		Cheeky,
		Flamingo,
		Dolly,
		Blush,
		Urchin,
		Mochi,
		Lilac,
		Prince,
		Violet,
		Mauve,
		Grape,
		Plum,
		Orchid,
		Jelly,
		Charple,
		Hazy,
		Ox,
		Sapphire,
		Guppy,
		Oceania,
		Thunder,
		Anchovy,
		Damson,
		Malibu,
		Sardine,
		Zinc,
		Turtle,
		Lichen,
		Guac,
		Julep,
		Bok,
		Mustard,
		Citron,
		Zest,
		Pepper,
		Barbeque,
		Charcoal,
		Iron,
		Oyster,
		Squid,
		Smoke,
		Ash,
		Salt,
		Butter,
	}
}

// IsPrimary indicates which colors are part of the core palette.
func (k Key) IsPrimary() bool {
	return slices.Contains([]Key{
		Charple,
		Dolly,
		Julep,
		Zest,
		Butter,
	}, k)
}

// IsSecondary indicates which colors are part of the secondary palette.
func (k Key) IsSecondary() bool {
	return slices.Contains([]Key{
		Hazy,
		Blush,
		Bok,
	}, k)
}

// IsTertiary indicates which colors are part of the tertiary palette.
func (k Key) IsTertiary() bool {
	return slices.Contains([]Key{
		Turtle,
		Malibu,
		Violet,
		Tuna,
		Coral,
		Uni,
	}, k)
}

// BlendColors returns a slice of colors blended between the given keys.
// Blending is done as Hcl to stay in gamut.
func BlendColors(size int, keys ...Key) []color.Color {
	if len(keys) < 2 {
		return nil
	}

	stops := make([]colorful.Color, len(keys))
	for i, k := range keys {
		stops[i], _ = colorful.Hex(k.Hex())
	}

	numSegments := len(stops) - 1
	if numSegments == 0 {
		return nil
	}
	blended := make([]color.Color, 0, size)

	// Calculate how many colors each segment should have.
	segmentSizes := make([]int, numSegments)
	baseSize := size / numSegments
	remainder := size % numSegments

	// Distribute the remainder across segments.
	for i := range numSegments {
		segmentSizes[i] = baseSize
		if i < remainder {
			segmentSizes[i]++
		}
	}

	// Generate colors for each segment.
	for i := range numSegments {
		c1 := stops[i]
		c2 := stops[i+1]
		segmentSize := segmentSizes[i]

		for j := range segmentSize {
			if segmentSize == 0 {
				continue
			}
			t := float64(j) / float64(segmentSize)
			c := c1.BlendHcl(c2, t)
			blended = append(blended, c)
		}
	}

	return blended
}
