package main

import "fmt"

func greeting(name string) string {
	names := []string{}
	return fmt.Sprintf("hello, %s", names[0])
}

func main() {
	fmt.Println(greeting("world"))
	fmt.Println("startup ok")
}
