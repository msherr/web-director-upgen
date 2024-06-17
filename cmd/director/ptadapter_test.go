package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetObsCertificates(t *testing.T) {

	// Call the function being tested
	m := getObsCertificates(123)

	for k, v := range m {
		t.Logf("Key: %s, Value: %v", k, v)
	}

	// TODO: Add assertions to verify the expected behavior of the function
	t.Logf("TestGetObsCertificates completed successfully")
}

func TestGetObsCertificatePart(t *testing.T) {

	certNums := []int{1, 2000, 3000, 4000, 5000}

	for _, num := range certNums {
		m := getObsCertificates(num)
		r := getObsCertificatePart(m["obfs4_bridgeline.txt"])
		assert.NotEqual(t, r, "")
	}
}
