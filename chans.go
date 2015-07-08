
package main

import "fmt"


func ToStream(arr []int) chan interface{} {
	out := make(chan interface{})
	go func() {
		for _, x := range arr {
			out <- x
		}
		close(out)
	}()
	return out
}

func Map(in chan interface{}) chan interface{} {
	out := make(chan interface{})
	go func() {
		for x := range in {
			out <- x.(int) + 1
		}
		close(out)
	}()
	return out
}

func FlatMap(in chan interface{}) chan interface{} {
	out := make(chan interface{})
	go func() {
		for x := range in {
			tmp := ToStream([]int{1,2,3})
			for a := range tmp {
				out <- a.(int) + x.(int)
			}
		}
		close(out)
	}()
	return out
}


func main1() {
//	data := []int {1,7,3,4}

	ch1 := make(chan interface{})
	go func() {
		ch1 <- 10
		ch1 <- 14
		ch1 <- 13
		ch1 <- 12
		close(ch1)
	}()

//	ch1 := ToStream(data)
	res0 := Map(ch1)
	res := FlatMap(res0)


	fmt.Println("chan:")
	for x := range res {
		fmt.Println(x)
	}

}
