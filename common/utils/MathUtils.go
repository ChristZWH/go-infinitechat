package utils

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
)

func RandomNumber(l int) (string, error) {
	// max := int(math.Pow(10, float64(len)))

	maxN := big.NewInt(int64(math.Pow10(l)))
	randN, err := rand.Int(rand.Reader, maxN)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", l, randN), nil
}
