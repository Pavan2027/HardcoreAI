// Package utils contains project-wide utility functions.
package utils

import (
	"unicode"

	"github.com/pkoukk/tiktoken-go"
)

// CountTokens returns the number of tokens in a string using cl100k_base (GPT-4 / GPT-3.5) encoding.
// It has a highly robust fallback to character/word-based estimation if tiktoken fails or has network issues
// when downloading cl100k_base files in an offline or firewalled environment.
func CountTokens(text string) int {
	tke, err := tiktoken.GetEncoding("cl100k_base")
	if err == nil {
		tokenized := tke.Encode(text, nil, nil)
		return len(tokenized)
	}

	// Fallback estimation (GPT-4 average token is ~4 characters or ~0.75 words)
	// This ensures the application never crashes/fails if offline or unable to download cl100k_base.
	words := 0
	chars := 0
	inWord := false
	for _, r := range text {
		chars++
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			inWord = true
			words++
		}
	}
	
	// Estimate using average of character-based and word-based count
	estFromChars := int(float64(chars) / 4.0)
	estFromWords := int(float64(words) / 0.75)
	
	if estFromChars > estFromWords {
		return estFromChars
	}
	if estFromWords == 0 {
		return 1
	}
	return estFromWords
}
