package main

import (
	"fmt"
	"net/url"
	"strings"
)

// Converts relative URL into absolute URL, and snips out the #whatever part
func cleanUrl(anchor *url.URL, rel string) (string, error) {
	// Filtering URL
	if strings.HasPrefix(rel, "javascript:") {
		return "", fmt.Errorf("can't clean URL beginning with javascript")
	}

	// Clean URL
	ref, err := url.Parse(rel)
	if err != nil {
		fmt.Println("Could not parse URL:", rel)
		return "", err
	}
	abs := anchor.ResolveReference(ref)
	abs.Fragment = ""
	return fmt.Sprint(abs), nil
}
