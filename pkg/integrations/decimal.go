package integrations

import (
	"encoding/json"
	"math/big"
	"strings"
)

func TrimDecimal(r *big.Rat) string {
	s := r.FloatString(12)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}

func PerMTok(tokens int64) string {
	return TrimDecimal(new(big.Rat).SetFrac(big.NewInt(tokens), big.NewInt(1_000_000)))
}

func UnitPricePerMTok(cost string, tokens int64) (string, bool) {
	c, ok := new(big.Rat).SetString(cost)
	if !ok || c.Sign() == 0 || tokens == 0 {
		return "", false
	}
	return TrimDecimal(new(big.Rat).Mul(c, big.NewRat(1_000_000, tokens))), true
}

func NonZero(n json.Number) bool {
	if n == "" {
		return false
	}
	f, err := n.Float64()
	return err == nil && f != 0
}
