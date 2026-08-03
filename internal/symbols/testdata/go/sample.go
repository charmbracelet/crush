package sample

import "fmt"

// Greet returns a greeting message.
func Greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

// Add adds two integers.
func Add(a, b int) int {
	return a + b
}

type Calculator struct {
	Value int
}

func (c *Calculator) Multiply(x int) int {
	c.Value *= x
	return c.Value
}
