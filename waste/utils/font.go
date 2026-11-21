package utils

// Stylize converts Latin letters to mathematical sans-serif italic characters.
// It is intentionally limited to plain UI strings so HTML tags, URLs and
// Telegram placeholders are not modified.
func Stylize(s string) string {
	out := make([]rune, 0, len([]rune(s)))
	for _, ch := range s {
		switch {
		case ch >= 'A' && ch <= 'Z':
			out = append(out, rune(0x1D608)+ch-'A')
		case ch >= 'a' && ch <= 'z':
			out = append(out, rune(0x1D622)+ch-'a')
		default:
			out = append(out, ch)
		}
	}
	return string(out)
}

// StylizeHTML applies Stylize to visible text while leaving HTML tags and
// entities untouched.
func StylizeHTML(s string) string {
	out := make([]rune, 0, len([]rune(s)))
	r := []rune(s)
	for i := 0; i < len(r); {
		if r[i] == '<' {
			j := i
			for j < len(r) && r[j] != '>' {
				j++
			}
			if j < len(r) {
				out = append(out, r[i:j+1]...)
				i = j + 1
				continue
			}
		}
		if r[i] == '&' {
			j := i
			for j < len(r) && r[j] != ';' && j-i < 12 {
				j++
			}
			if j < len(r) && r[j] == ';' {
				out = append(out, r[i:j+1]...)
				i = j + 1
				continue
			}
		}
		ch := r[i]
		switch {
		case ch >= 'A' && ch <= 'Z':
			out = append(out, rune(0x1D608)+ch-'A')
		case ch >= 'a' && ch <= 'z':
			out = append(out, rune(0x1D622)+ch-'a')
		default:
			out = append(out, ch)
		}
		i++
	}
	return string(out)
}
