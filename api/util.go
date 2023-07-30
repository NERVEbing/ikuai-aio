package api

import (
	"crypto/md5"
	"encoding/hex"
	"net"
	"strings"
	"unicode"
)

func toMD5(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func chunkSliceStr(slice []string, size int) [][]string {
	if size < 1 {
		return nil
	}
	chunks := make([][]string, 0)
	for i := 0; i < len(slice); i += size {
		end := i + size
		if end > len(slice) {
			end = len(slice)
		}
		chunk := slice[i:end]
		chunks = append(chunks, chunk)
	}

	return chunks
}

func parseIPv4(s string) string {
	ip := net.ParseIP(s)
	if ip != nil && ip.To4() != nil {
		s = ip.String()
	} else {
		ip, cidr, err := net.ParseCIDR(s)
		if err != nil || ip.To4() == nil {
			return ""
		}
		s = cidr.String()
	}

	return s
}

func isValidDomain(s string) bool {
	if len(s) < 1 || len(s) > 255 {
		return false
	}
	for i, char := range s {
		if !isValidDomainCharacter(char, i, s) {
			return false
		}
	}

	return true
}

func isValidDomainCharacter(char rune, index int, domain string) bool {
	if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' {
		return true
	}
	if char == '.' && index > 0 && index < len(domain)-1 && !strings.HasPrefix(domain, ".") && !strings.HasSuffix(domain, ".") {
		return true
	}

	return false
}
