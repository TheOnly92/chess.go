package chess

import (
	"fmt"
	"strings"
)

func popCount(b uint64) int {
	count := 0
	for b > 0 {
		count++
		b = b & (b - 1)
	}
	return count
}

func bitScan(b uint64, n int) int {
	str := fmt.Sprintf("%b", b)
	r := strings.LastIndex(str[:(len(str)-n)], "1")
	if r == -1 {
		return -1
	}
	return len(str) - r - 1
}
