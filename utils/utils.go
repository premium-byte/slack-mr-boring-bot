package utils

import (
	"math/rand"
	"time"
)

func Shuffle(slice []string) {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	rand.Shuffle(len(slice), func(i, j int) {
		slice[i], slice[j] = slice[j], slice[i]
	})
}

func Difference(A, B []string) []string {
	// Create a map from list B
	bMap := make(map[string]struct{})
	for _, item := range B {
		bMap[item] = struct{}{}
	}

	// Find elements in A that are not in B
	var diff []string
	for _, item := range A {
		if _, found := bMap[item]; !found {
			diff = append(diff, item)
		}
	}

	return diff
}
