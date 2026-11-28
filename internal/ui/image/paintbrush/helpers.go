package paintbrush

import (
	"fmt"
	"math"
)

type Vec4 struct {
	R, G, B, A float64
}

func (v Vec4) Add(other Vec4) Vec4 {
	return Vec4{v.R + other.R, v.G + other.G, v.B + other.B, v.A + other.A}
}

func (v Vec4) Sub(other Vec4) Vec4 {
	return Vec4{v.R - other.R, v.G - other.G, v.B - other.B, v.A - other.A}
}

func (v Vec4) Mul(f float64) Vec4 {
	return Vec4{v.R * f, v.G * f, v.B * f, v.A * f}
}

func (v Vec4) Div(f float64) Vec4 {
	return Vec4{v.R / f, v.G / f, v.B / f, v.A / f}
}

func (v Vec4) Sum() float64 {
	return v.R + v.G + v.B + v.A
}

func (v Vec4) Dot(other Vec4) float64 {
	return v.R*other.R + v.G*other.G + v.B*other.B + v.A*other.A
}

func (v Vec4) Abs() Vec4 {
	return Vec4{
		R: math.Abs(v.R),
		G: math.Abs(v.G),
		B: math.Abs(v.B),
		A: math.Abs(v.A),
	}
}

func (v Vec4) ToPixel() Pixel {
	return Pixel{
		R: uint8(v.R * 255),
		G: uint8(v.G * 255),
		B: uint8(v.B * 255),
		A: uint8(v.A * 255),
	}
}

type Pixel struct {
	R, G, B, A uint8
}

func (p Pixel) AnsiColor() string {
	return fmt.Sprintf("2;%d;%d;%d", p.R, p.G, p.B)
}

func (p Pixel) AnsiBg() string {
	return "\033[48;" + p.AnsiColor() + "m"
}

func (p Pixel) AnsiFg() string {
	return "\033[38;" + p.AnsiColor() + "m"
}
