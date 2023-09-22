package main

import (
	"strings"
)

// StringSliceVar is a custom type that implements the flag.Value interface
// to store a list of strings.
type StringSliceVar []string

// String returns a string representation of the StringSliceVar type.
func (ss *StringSliceVar) String() string {
	return strings.Join(*ss, ", ")
}

// Set appends a value to the StringSliceVar.
func (ss *StringSliceVar) Set(value string) error {
	*ss = append(*ss, value)
	return nil
}
