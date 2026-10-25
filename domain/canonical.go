package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// Digests serve artifact integrity and reproducible transformation records.
// The domain ships a deterministic, portable hasher; cryptographic hashing is
// available too, and strength is recorded on every digest so that a weak
// digest can never masquerade as an audit-grade one.

const (
	DigestFNV1a64 = "fnv1a64"
	DigestSHA256  = "sha256"

	StrengthCryptographic    = "cryptographic"
	StrengthNonCryptographic = "non-cryptographic"
)

var DigestAlgorithms = []string{DigestFNV1a64, DigestSHA256}
var DigestStrengths = []string{StrengthCryptographic, StrengthNonCryptographic}

type Digest struct {
	Algorithm string `json:"algorithm"`
	Strength  string `json:"strength"`
	Value     string `json:"value"`
	// What the digest was taken over — disambiguates the coverage of the hash.
	Covers string `json:"covers"`
}

type Hasher interface {
	Algorithm() string
	Strength() string
	Digest(canonical string) string
}

var StrengthByAlgorithm = map[string]string{
	DigestFNV1a64: StrengthNonCryptographic,
	DigestSHA256:  StrengthCryptographic,
}

type fnvHasher struct{}

func (fnvHasher) Algorithm() string              { return DigestFNV1a64 }
func (fnvHasher) Strength() string               { return StrengthNonCryptographic }
func (fnvHasher) Digest(canonical string) string { return FNV1a64(canonical) }

type shaHasher struct{}

func (shaHasher) Algorithm() string { return DigestSHA256 }
func (shaHasher) Strength() string  { return StrengthCryptographic }
func (shaHasher) Digest(canonical string) string {
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

// FNV1a64Hasher is the deterministic hasher used by the domain by default.
// Non-cryptographic by construction and always labelled as such.
var FNV1a64Hasher Hasher = fnvHasher{}

// SHA256Hasher is the cryptographic hasher used for audit trails.
var SHA256Hasher Hasher = shaHasher{}

// FNV1a64 is FNV-1a, 64-bit, over the UTF-8 bytes, as 16 lowercase hex chars.
func FNV1a64(input string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(input))
	return fmt.Sprintf("%016x", h.Sum64())
}

// UTF8ByteLength is the UTF-8 byte length of a string.
func UTF8ByteLength(input string) int {
	return len(input)
}

func DigestOf(hasher Hasher, value any, covers string) Digest {
	return Digest{
		Algorithm: hasher.Algorithm(),
		Strength:  hasher.Strength(),
		Value:     hasher.Digest(Canonicalize(value)),
		Covers:    covers,
	}
}

func DigestsEqual(a, b Digest) bool {
	return a.Algorithm == b.Algorithm && a.Value == b.Value
}

// ToGeneric converts any JSON-serialisable value into the generic tree
// (map[string]any, []any, string, json.Number, bool, nil). Every structural
// walk — canonicalisation, forbidden-key scans, research atoms — runs over
// this tree, so it sees exactly what would be serialised.
func ToGeneric(value any) any {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("canonicalize: value is not representable: %v", err))
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var out any
	if err := decoder.Decode(&out); err != nil {
		panic(fmt.Sprintf("canonicalize: %v", err))
	}
	return out
}

// Canonicalize renders canonical JSON: object keys sorted, absent members
// omitted, no reliance on key insertion order. Two structurally identical
// values always produce the same string, and therefore the same digest.
func Canonicalize(value any) string {
	var b strings.Builder
	writeCanonical(&b, ToGeneric(value))
	return b.String()
}

func writeCanonical(b *strings.Builder, node any) {
	switch v := node.(type) {
	case nil:
		b.WriteString("null")
	case string:
		b.WriteString(JSQuote(v))
	case bool:
		if v {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			panic(fmt.Sprintf("canonicalize: bad number %q", v))
		}
		b.WriteString(JSNumber(f))
	case []any:
		b.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			writeCanonical(b, item)
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(JSQuote(key))
			b.WriteByte(':')
			writeCanonical(b, v[key])
		}
		b.WriteByte('}')
	default:
		panic(fmt.Sprintf("canonicalize: unsupported value of type %T", node))
	}
}

// JSQuote quotes a string exactly as JSON.stringify does: only the quote,
// the backslash and control characters are escaped.
func JSQuote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// JSNumber formats a number the way JavaScript's String(n) and
// JSON.stringify(n) do (shortest round-trip representation).
func JSNumber(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		panic("canonicalize: non-finite numbers are not representable")
	}
	if f == 0 {
		return "0"
	}
	abs := math.Abs(f)
	if abs >= 1e21 || abs < 1e-6 {
		s := strconv.FormatFloat(f, 'e', -1, 64)
		// Go writes e-07 / e+21; JavaScript writes e-7 / e+21.
		mantissa, exponent, _ := strings.Cut(s, "e")
		sign := exponent[0]
		digits := strings.TrimLeft(exponent[1:], "0")
		if digits == "" {
			digits = "0"
		}
		return mantissa + "e" + string(sign) + digits
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// JSFixed formats like Number.prototype.toFixed: the exact binary value is
// rounded to the nearest representation, and exact ties round away from zero
// (strconv would round them to even).
func JSFixed(f float64, digits int) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	negative := f < 0
	exact := new(big.Float).SetPrec(2048).SetFloat64(math.Abs(f))
	scale := new(big.Float).SetPrec(2048).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil))
	exact.Mul(exact, scale)
	whole, _ := exact.Int(nil)
	fraction := new(big.Float).SetPrec(2048).Sub(exact, new(big.Float).SetPrec(2048).SetInt(whole))
	if fraction.Cmp(big.NewFloat(0.5)) >= 0 {
		whole.Add(whole, big.NewInt(1))
	}
	text := whole.String()
	if digits > 0 {
		if len(text) <= digits {
			text = strings.Repeat("0", digits-len(text)+1) + text
		}
		text = text[:len(text)-digits] + "." + text[len(text)-digits:]
	}
	if negative && strings.Trim(text, "0.") != "" {
		text = "-" + text
	}
	return text
}
