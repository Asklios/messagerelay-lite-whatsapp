package util

import (
	"bytes"
)

func ConvertHTMLToWAStyle(input string) string {
	var output bytes.Buffer
	tagStarted := false
	closingTag := false
	var tag string
	for index, char := range input {
		switch true {
		case char == '<':
			tagStarted = true
		case char == '>':
			switch tag {
			case "em":
				output.WriteRune('_')
			case "strong":
				output.WriteRune('*')
			case "s":
				output.WriteRune('~')
			case "p":
				if closingTag && (index+1) < len(input) {
					output.WriteRune('\n')
				}
			case "br":
				output.WriteRune('\n')
			}
			tag = ""
			tagStarted = false
			closingTag = false
		case char == '/':
			if !tagStarted {
				output.WriteRune(char)
			} else {
				closingTag = true
			}
		default:
			if tagStarted {
				tag += string(char)
			} else if char != 65279 {
				output.WriteRune(char)
			}
		}
	}
	return output.String()
}
