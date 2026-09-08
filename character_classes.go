package oneocr

import (
	"fmt"
	"strings"
	"unicode"
)

// CharacterClass selects writing-system characters, not the language of a line.
// Shared Han characters cannot distinguish Chinese from Japanese or Korean.
type CharacterClass string

const (
	CharactersHan    CharacterClass = "han"
	CharactersKana   CharacterClass = "kana"
	CharactersHangul CharacterClass = "hangul"
	CharactersLatin  CharacterClass = "latin"
	CharactersDigits CharacterClass = "digits"
)

func characterClassSet(classes []CharacterClass) (map[CharacterClass]bool, error) {
	if len(classes) == 0 {
		classes = []CharacterClass{CharactersHan, CharactersKana, CharactersHangul, CharactersLatin, CharactersDigits}
	}
	out := map[CharacterClass]bool{}
	for _, class := range classes {
		switch class {
		case CharactersHan, CharactersKana, CharactersHangul, CharactersLatin, CharactersDigits:
		default:
			return nil, fmt.Errorf("oneocr: unknown character class %q", class)
		}
		if out[class] {
			return nil, fmt.Errorf("oneocr: duplicate character class %q", class)
		}
		out[class] = true
	}
	return out, nil
}
func (a *alphabet) setClasses(classes []CharacterClass) error {
	enabled, err := characterClassSet(classes)
	if err != nil {
		return err
	}
	a.allowed = make([]bool, len(a.characters))
	for i, token := range a.characters {
		// Preserve special-token validation in the decoder.
		if strings.HasPrefix(token, "<") && strings.HasSuffix(token, ">") {
			a.allowed[i] = true
			continue
		}
		if mapped, ok := a.composites[token]; ok {
			token = mapped
		}
		allow := true
		for _, r := range token {
			var class CharacterClass
			switch {
			case unicode.Is(unicode.Han, r):
				class = CharactersHan
			case unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || r == 'ー' || r == 'ｰ':
				class = CharactersKana
			case unicode.Is(unicode.Hangul, r):
				class = CharactersHangul
			case unicode.Is(unicode.Latin, r):
				class = CharactersLatin
			case unicode.IsDigit(r):
				class = CharactersDigits
			case unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.IsMark(r):
				continue
			default:
				allow = false
			}
			if class != "" && !enabled[class] {
				allow = false
			}
		}
		a.allowed[i] = allow
	}
	a.allowed[a.blank] = true
	return nil
}
func (a *alphabet) tokenMask() []bool {
	if len(a.allowed) == len(a.characters) {
		return a.allowed
	}
	// Extended reference dictionaries have no production character restriction.
	a.allowed = make([]bool, len(a.characters))
	for i := range a.allowed {
		a.allowed[i] = true
	}
	return a.allowed
}
