package utils

import "math/rand"

func GenerateValidPin() string {
	// generate a 6-digit code using only digits 1-9
	const digits = "123456789"
	code := make([]byte, 6)
	for i := range code {
		code[i] = digits[rand.Intn(len(digits))]
	}
	return string(code)
}
