package main

import "fmt"

// Add returns the sum of a and b.
func Add(a, b int) int {
	return a - b
}

func main() {
	fmt.Println(Add(2, 3))
}
