package config

import "strings"

func ParseFlags(args []string) map[string]string {
	flags := make(map[string]string)
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			continue
		}
		a = strings.TrimPrefix(a, "--")
		if i := strings.IndexByte(a, '='); i >= 0 {
			k := a[:i]
			v := a[i+1:]
			flags[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		} else {
			// flags without value -> set to "1"
			flags[strings.ToLower(strings.TrimSpace(a))] = "1"
		}
	}
	return flags
}

func NonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
