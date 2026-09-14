package main

import "fmt"

func main() {
	var counts map[string]int
	counts["apple"] = 2
	counts["banana"] = 3
	fmt.Println("apple", counts["apple"])
	fmt.Println("banana", counts["banana"])
}
