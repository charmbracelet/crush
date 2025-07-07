package util

import "unicode"

var cjkRadicalsSupplement = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x2E80, 0x2EFF, 1},
	},
}

var kangxiRadicals = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x2F00, 0x2FDF, 1},
	},
}

var ideographicDescriptionCharacters = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x2FF0, 0x2FFF, 1},
	},
}

var cjkSymbolsAndPunctuation = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x3000, 0x303F, 1},
	},
}

var hiragana = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x3040, 0x309F, 1},
	},
}

var katakana = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x30A0, 0x30FF, 1},
	},
}

var kanbun = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x3130, 0x318F, 1},
		{0x3190, 0x319F, 1},
	},
}

var cjkStrokes = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x31C0, 0x31EF, 1},
	},
}

var katakanaPhoneticExtensions = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x31F0, 0x31FF, 1},
	},
}

var cjkCompatibility = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x3300, 0x33FF, 1},
	},
}

var cjkUnifiedIdeographsExtensionA = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x3400, 0x4DBF, 1},
	},
}

var cjkUnifiedIdeographs = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x4E00, 0x9FFF, 1},
	},
}

var yiSyllables = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0xA000, 0xA48F, 1},
	},
}

var yiRadicals = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0xA490, 0xA4CF, 1},
	},
}

var cjkCompatibilityIdeographs = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0xF900, 0xFAFF, 1},
	},
}

var verticalForms = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0xFE10, 0xFE1F, 1},
	},
}

var cjkCompatibilityForms = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0xFE30, 0xFE4F, 1},
	},
}

var smallFormVariants = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0xFE50, 0xFE6F, 1},
	},
}

var halfwidthAndFullwidthForms = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0xFF00, 0xFFEF, 1},
	},
}

var kanaSupplement = &unicode.RangeTable{
	R32: []unicode.Range32{
		{0x1B000, 0x1B0FF, 1},
	},
}

var kanaExtendedA = &unicode.RangeTable{
	R32: []unicode.Range32{
		{0x1B100, 0x1B12F, 1},
	},
}

var smallKanaExtension = &unicode.RangeTable{
	R32: []unicode.Range32{
		{0x1B130, 0x1B16F, 1},
	},
}

var cjkUnifiedIdeographsExtensionB = &unicode.RangeTable{
	R32: []unicode.Range32{
		{0x20000, 0x2A6DF, 1},
	},
}

var cjkUnifiedIdeographsExtensionC = &unicode.RangeTable{
	R32: []unicode.Range32{
		{0x2A700, 0x2B73F, 1},
	},
}

var cjkUnifiedIdeographsExtensionD = &unicode.RangeTable{
	R32: []unicode.Range32{
		{0x2B740, 0x2B81F, 1},
	},
}

var cjkUnifiedIdeographsExtensionE = &unicode.RangeTable{
	R32: []unicode.Range32{
		{0x2B820, 0x2CEAF, 1},
	},
}

var cjkUnifiedIdeographsExtensionF = &unicode.RangeTable{
	R32: []unicode.Range32{
		{0x2CEB0, 0x2EBEF, 1},
	},
}

var cjkCompatibilityIdeographsSupplement = &unicode.RangeTable{
	R32: []unicode.Range32{
		{0x2F800, 0x2FA1F, 1},
	},
}

var cjkUnifiedIdeographsExtensionG = &unicode.RangeTable{
	R32: []unicode.Range32{
		{0x30000, 0x3134F, 1},
	},
}

// IsEastAsianWideRune returns trhe if the given rune is an east asian wide character, otherwise false.
func IsEastAsianWideRune(r rune) bool {
	return unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Lm, r) ||
		unicode.Is(unicode.Hangul, r) ||
		unicode.Is(cjkSymbolsAndPunctuation, r)
}

// IsSpaceDiscardingUnicodeRune returns true if the given rune is space-discarding unicode character, otherwise false.
// See https://www.w3.org/TR/2020/WD-css-text-3-20200429/#space-discard-set
func IsSpaceDiscardingUnicodeRune(r rune) bool {
	return unicode.Is(cjkRadicalsSupplement, r) ||
		unicode.Is(kangxiRadicals, r) ||
		unicode.Is(ideographicDescriptionCharacters, r) ||
		unicode.Is(cjkSymbolsAndPunctuation, r) ||
		unicode.Is(hiragana, r) ||
		unicode.Is(katakana, r) ||
		unicode.Is(kanbun, r) ||
		unicode.Is(cjkStrokes, r) ||
		unicode.Is(katakanaPhoneticExtensions, r) ||
		unicode.Is(cjkCompatibility, r) ||
		unicode.Is(cjkUnifiedIdeographsExtensionA, r) ||
		unicode.Is(cjkUnifiedIdeographs, r) ||
		unicode.Is(yiSyllables, r) ||
		unicode.Is(yiRadicals, r) ||
		unicode.Is(cjkCompatibilityIdeographs, r) ||
		unicode.Is(verticalForms, r) ||
		unicode.Is(cjkCompatibilityForms, r) ||
		unicode.Is(smallFormVariants, r) ||
		unicode.Is(halfwidthAndFullwidthForms, r) ||
		unicode.Is(kanaSupplement, r) ||
		unicode.Is(kanaExtendedA, r) ||
		unicode.Is(smallKanaExtension, r) ||
		unicode.Is(cjkUnifiedIdeographsExtensionB, r) ||
		unicode.Is(cjkUnifiedIdeographsExtensionC, r) ||
		unicode.Is(cjkUnifiedIdeographsExtensionD, r) ||
		unicode.Is(cjkUnifiedIdeographsExtensionE, r) ||
		unicode.Is(cjkUnifiedIdeographsExtensionF, r) ||
		unicode.Is(cjkCompatibilityIdeographsSupplement, r) ||
		unicode.Is(cjkUnifiedIdeographsExtensionG, r)
}

// EastAsianWidth returns the east asian width of the given rune.
// See https://www.unicode.org/reports/tr11/tr11-36.html
func EastAsianWidth(r rune) string {
	switch {
	case r == 0x3000,
		(0xFF01 <= r && r <= 0xFF60),
		(0xFFE0 <= r && r <= 0xFFE6):
		return "F"

	case r == 0x20A9,
		(0xFF61 <= r && r <= 0xFFBE),
		(0xFFC2 <= r && r <= 0xFFC7),
		(0xFFCA <= r && r <= 0xFFCF),
		(0xFFD2 <= r && r <= 0xFFD7),
		(0xFFDA <= r && r <= 0xFFDC),
		(0xFFE8 <= r && r <= 0xFFEE):
		return "H"

	case (0x1100 <= r && r <= 0x115F),
		(0x11A3 <= r && r <= 0x11A7),
		(0x11FA <= r && r <= 0x11FF),
		(0x2329 <= r && r <= 0x232A),
		(0x2E80 <= r && r <= 0x2E99),
		(0x2E9B <= r && r <= 0x2EF3),
		(0x2F00 <= r && r <= 0x2FD5),
		(0x2FF0 <= r && r <= 0x2FFB),
		(0x3001 <= r && r <= 0x303E),
		(0x3041 <= r && r <= 0x3096),
		(0x3099 <= r && r <= 0x30FF),
		(0x3105 <= r && r <= 0x312D),
		(0x3131 <= r && r <= 0x318E),
		(0x3190 <= r && r <= 0x31BA),
		(0x31C0 <= r && r <= 0x31E3),
		(0x31F0 <= r && r <= 0x321E),
		(0x3220 <= r && r <= 0x3247),
		(0x3250 <= r && r <= 0x32FE),
		(0x3300 <= r && r <= 0x4DBF),
		(0x4E00 <= r && r <= 0xA48C),
		(0xA490 <= r && r <= 0xA4C6),
		(0xA960 <= r && r <= 0xA97C),
		(0xAC00 <= r && r <= 0xD7A3),
		(0xD7B0 <= r && r <= 0xD7C6),
		(0xD7CB <= r && r <= 0xD7FB),
		(0xF900 <= r && r <= 0xFAFF),
		(0xFE10 <= r && r <= 0xFE19),
		(0xFE30 <= r && r <= 0xFE52),
		(0xFE54 <= r && r <= 0xFE66),
		(0xFE68 <= r && r <= 0xFE6B),
		(0x1B000 <= r && r <= 0x1B001),
		(0x1F200 <= r && r <= 0x1F202),
		(0x1F210 <= r && r <= 0x1F23A),
		(0x1F240 <= r && r <= 0x1F248),
		(0x1F250 <= r && r <= 0x1F251),
		(0x20000 <= r && r <= 0x2F73F),
		(0x2B740 <= r && r <= 0x2FFFD),
		(0x30000 <= r && r <= 0x3FFFD):
		return "W"

	case (0x0020 <= r && r <= 0x007E),
		(0x00A2 <= r && r <= 0x00A3),
		(0x00A5 <= r && r <= 0x00A6),
		r == 0x00AC,
		r == 0x00AF,
		(0x27E6 <= r && r <= 0x27ED),
		(0x2985 <= r && r <= 0x2986):
		return "Na"

	case (0x00A1 == r),
		(0x00A4 == r),
		(0x00A7 <= r && r <= 0x00A8),
		(0x00AA == r),
		(0x00AD <= r && r <= 0x00AE),
		(0x00B0 <= r && r <= 0x00B4),
		(0x00B6 <= r && r <= 0x00BA),
		(0x00BC <= r && r <= 0x00BF),
		(0x00C6 == r),
		(0x00D0 == r),
		(0x00D7 <= r && r <= 0x00D8),
		(0x00DE <= r && r <= 0x00E1),
		(0x00E6 == r),
		(0x00E8 <= r && r <= 0x00EA),
		(0x00EC <= r && r <= 0x00ED),
		(0x00F0 == r),
		(0x00F2 <= r && r <= 0x00F3),
		(0x00F7 <= r && r <= 0x00FA),
		(0x00FC == r),
		(0x00FE == r),
		(0x0101 == r),
		(0x0111 == r),
		(0x0113 == r),
		(0x011B == r),
		(0x0126 <= r && r <= 0x0127),
		(0x012B == r),
		(0x0131 <= r && r <= 0x0133),
		(0x0138 == r),
		(0x013F <= r && r <= 0x0142),
		(0x0144 == r),
		(0x0148 <= r && r <= 0x014B),
		(0x014D == r),
		(0x0152 <= r && r <= 0x0153),
		(0x0166 <= r && r <= 0x0167),
		(0x016B == r),
		(0x01CE == r),
		(0x01D0 == r),
		(0x01D2 == r),
		(0x01D4 == r),
		(0x01D6 == r),
		(0x01D8 == r),
		(0x01DA == r),
		(0x01DC == r),
		(0x0251 == r),
		(0x0261 == r),
		(0x02C4 == r),
		(0x02C7 == r),
		(0x02C9 <= r && r <= 0x02CB),
		(0x02CD == r),
		(0x02D0 == r),
		(0x02D8 <= r && r <= 0x02DB),
		(0x02DD == r),
		(0x02DF == r),
		(0x0300 <= r && r <= 0x036F),
		(0x0391 <= r && r <= 0x03A1),
		(0x03A3 <= r && r <= 0x03A9),
		(0x03B1 <= r && r <= 0x03C1),
		(0x03C3 <= r && r <= 0x03C9),
		(0x0401 == r),
		(0x0410 <= r && r <= 0x044F),
		(0x0451 == r),
		(0x2010 == r),
		(0x2013 <= r && r <= 0x2016),
		(0x2018 <= r && r <= 0x2019),
		(0x201C <= r && r <= 0x201D),
		(0x2020 <= r && r <= 0x2022),
		(0x2024 <= r && r <= 0x2027),
		(0x2030 == r),
		(0x2032 <= r && r <= 0x2033),
		(0x2035 == r),
		(0x203B == r),
		(0x203E == r),
		(0x2074 == r),
		(0x207F == r),
		(0x2081 <= r && r <= 0x2084),
		(0x20AC == r),
		(0x2103 == r),
		(0x2105 == r),
		(0x2109 == r),
		(0x2113 == r),
		(0x2116 == r),
		(0x2121 <= r && r <= 0x2122),
		(0x2126 == r),
		(0x212B == r),
		(0x2153 <= r && r <= 0x2154),
		(0x215B <= r && r <= 0x215E),
		(0x2160 <= r && r <= 0x216B),
		(0x2170 <= r && r <= 0x2179),
		(0x2189 == r),
		(0x2190 <= r && r <= 0x2199),
		(0x21B8 <= r && r <= 0x21B9),
		(0x21D2 == r),
		(0x21D4 == r),
		(0x21E7 == r),
		(0x2200 == r),
		(0x2202 <= r && r <= 0x2203),
		(0x2207 <= r && r <= 0x2208),
		(0x220B == r),
		(0x220F == r),
		(0x2211 == r),
		(0x2215 == r),
		(0x221A == r),
		(0x221D <= r && r <= 0x2220),
		(0x2223 == r),
		(0x2225 == r),
		(0x2227 <= r && r <= 0x222C),
		(0x222E == r),
		(0x2234 <= r && r <= 0x2237),
		(0x223C <= r && r <= 0x223D),
		(0x2248 == r),
		(0x224C == r),
		(0x2252 == r),
		(0x2260 <= r && r <= 0x2261),
		(0x2264 <= r && r <= 0x2267),
		(0x226A <= r && r <= 0x226B),
		(0x226E <= r && r <= 0x226F),
		(0x2282 <= r && r <= 0x2283),
		(0x2286 <= r && r <= 0x2287),
		(0x2295 == r),
		(0x2299 == r),
		(0x22A5 == r),
		(0x22BF == r),
		(0x2312 == r),
		(0x2460 <= r && r <= 0x24E9),
		(0x24EB <= r && r <= 0x254B),
		(0x2550 <= r && r <= 0x2573),
		(0x2580 <= r && r <= 0x258F),
		(0x2592 <= r && r <= 0x2595),
		(0x25A0 <= r && r <= 0x25A1),
		(0x25A3 <= r && r <= 0x25A9),
		(0x25B2 <= r && r <= 0x25B3),
		(0x25B6 <= r && r <= 0x25B7),
		(0x25BC <= r && r <= 0x25BD),
		(0x25C0 <= r && r <= 0x25C1),
		(0x25C6 <= r && r <= 0x25C8),
		(0x25CB == r),
		(0x25CE <= r && r <= 0x25D1),
		(0x25E2 <= r && r <= 0x25E5),
		(0x25EF == r),
		(0x2605 <= r && r <= 0x2606),
		(0x2609 == r),
		(0x260E <= r && r <= 0x260F),
		(0x2614 <= r && r <= 0x2615),
		(0x261C == r),
		(0x261E == r),
		(0x2640 == r),
		(0x2642 == r),
		(0x2660 <= r && r <= 0x2661),
		(0x2663 <= r && r <= 0x2665),
		(0x2667 <= r && r <= 0x266A),
		(0x266C <= r && r <= 0x266D),
		(0x266F == r),
		(0x269E <= r && r <= 0x269F),
		(0x26BE <= r && r <= 0x26BF),
		(0x26C4 <= r && r <= 0x26CD),
		(0x26CF <= r && r <= 0x26E1),
		(0x26E3 == r),
		(0x26E8 <= r && r <= 0x26FF),
		(0x273D == r),
		(0x2757 == r),
		(0x2776 <= r && r <= 0x277F),
		(0x2B55 <= r && r <= 0x2B59),
		(0x3248 <= r && r <= 0x324F),
		(0xE000 <= r && r <= 0xF8FF),
		(0xFE00 <= r && r <= 0xFE0F),
		(0xFFFD == r),
		(0x1F100 <= r && r <= 0x1F10A),
		(0x1F110 <= r && r <= 0x1F12D),
		(0x1F130 <= r && r <= 0x1F169),
		(0x1F170 <= r && r <= 0x1F19A),
		(0xE0100 <= r && r <= 0xE01EF),
		(0xF0000 <= r && r <= 0xFFFFD),
		(0x100000 <= r && r <= 0x10FFFD):
		return "A"

	default:
		return "N"
	}
}
