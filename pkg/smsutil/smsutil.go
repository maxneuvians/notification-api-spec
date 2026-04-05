package smsutil

import "strings"

var gsmRunes = map[rune]struct{}{
	'@': {}, '£': {}, '$': {}, '¥': {}, 'è': {}, 'é': {}, 'ù': {}, 'ì': {}, 'ò': {}, 'Ç': {},
	'\n': {}, 'Ø': {}, 'ø': {}, '\r': {}, 'Å': {}, 'å': {}, 'Δ': {}, '_': {}, 'Φ': {}, 'Γ': {},
	'Λ': {}, 'Ω': {}, 'Π': {}, 'Ψ': {}, 'Σ': {}, 'Θ': {}, 'Ξ': {}, ' ': {}, '!': {}, '"': {},
	'#': {}, '¤': {}, '%': {}, '&': {}, '\'': {}, '(': {}, ')': {}, '*': {}, '+': {}, ',': {},
	'-': {}, '.': {}, '/': {}, '0': {}, '1': {}, '2': {}, '3': {}, '4': {}, '5': {}, '6': {},
	'7': {}, '8': {}, '9': {}, ':': {}, ';': {}, '<': {}, '=': {}, '>': {}, '?': {}, '¡': {},
	'A': {}, 'B': {}, 'C': {}, 'D': {}, 'E': {}, 'F': {}, 'G': {}, 'H': {}, 'I': {}, 'J': {},
	'K': {}, 'L': {}, 'M': {}, 'N': {}, 'O': {}, 'P': {}, 'Q': {}, 'R': {}, 'S': {}, 'T': {},
	'U': {}, 'V': {}, 'W': {}, 'X': {}, 'Y': {}, 'Z': {}, 'Ä': {}, 'Ö': {}, 'Ñ': {}, 'Ü': {},
	'§': {}, '¿': {}, 'a': {}, 'b': {}, 'c': {}, 'd': {}, 'e': {}, 'f': {}, 'g': {}, 'h': {},
	'i': {}, 'j': {}, 'k': {}, 'l': {}, 'm': {}, 'n': {}, 'o': {}, 'p': {}, 'q': {}, 'r': {},
	's': {}, 't': {}, 'u': {}, 'v': {}, 'w': {}, 'x': {}, 'y': {}, 'z': {}, 'ä': {}, 'ö': {},
	'ñ': {}, 'ü': {}, 'à': {}, '^': {}, '{': {}, '}': {}, '\\': {}, '[': {}, '~': {}, ']': {}, '|': {}, '€': {},
}

func Normalize(content string) string {
	var builder strings.Builder
	for _, r := range content {
		switch r {
		case '\t':
			builder.WriteByte(' ')
		case '\u200b', '\u200c', '\u200d', '\ufeff':
			continue
		case '\u2026':
			builder.WriteString("...")
		default:
			if _, ok := gsmRunes[r]; ok {
				builder.WriteRune(r)
				continue
			}
			if r >= 0 && r <= 127 {
				builder.WriteRune(r)
				continue
			}
			builder.WriteByte('?')
		}
	}
	return builder.String()
}

func ApplyPrefix(serviceName, content string, enabled bool) string {
	if !enabled {
		return content
	}
	name := strings.TrimSpace(serviceName)
	if name == "" {
		return content
	}
	return name + ": " + content
}