package main

import (
	"testing"
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
