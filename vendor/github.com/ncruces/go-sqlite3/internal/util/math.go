package util

import "math"

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func GCD(m, n int) int {
	for n != 0 {
		m, n = n, m%n
	}
	return abs(m)
}

func LCM(m, n int) int {
	if n == 0 {
		return 0
	}
	return abs(n) * (abs(m) / GCD(m, n))
}

// https://developer.nvidia.com/blog/lerp-faster-cuda/
func Lerp(v0, v1, t float64) float64 {
	return math.FMA(t, v1, math.FMA(-t, v0, v0))
}
