package main

import "strings"

func cleanWords(str string) string {
	badWords := map[string]bool {
		"kerfuffle": true,
		"sharbert": true,
		"fornax": true,
	}
	
	newWords := []string{}
	for word := range strings.SplitSeq(str, " "){
		if badWords[strings.ToLower(word)] {
			newWords = append(newWords, "****")
		} else {
			newWords = append(newWords, word)
		}
	}
	return strings.Join(newWords, " ")
}
