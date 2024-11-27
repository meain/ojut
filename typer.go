package main

import (
	"strings"
	"unicode"

	"github.com/micmonay/keybd_event"
)

func typeString(str string, kb keybd_event.KeyBonding) error {
	// Explicit mapping for a-z characters
	charToKeyCode := map[rune]int{
		'a': keybd_event.VK_A, 'b': keybd_event.VK_B, 'c': keybd_event.VK_C, 'd': keybd_event.VK_D,
		'e': keybd_event.VK_E, 'f': keybd_event.VK_F, 'g': keybd_event.VK_G, 'h': keybd_event.VK_H,
		'i': keybd_event.VK_I, 'j': keybd_event.VK_J, 'k': keybd_event.VK_K, 'l': keybd_event.VK_L,
		'm': keybd_event.VK_M, 'n': keybd_event.VK_N, 'o': keybd_event.VK_O, 'p': keybd_event.VK_P,
		'q': keybd_event.VK_Q, 'r': keybd_event.VK_R, 's': keybd_event.VK_S, 't': keybd_event.VK_T,
		'u': keybd_event.VK_U, 'v': keybd_event.VK_V, 'w': keybd_event.VK_W, 'x': keybd_event.VK_X,
		'y': keybd_event.VK_Y, 'z': keybd_event.VK_Z,

		'A': keybd_event.VK_A, 'B': keybd_event.VK_B, 'C': keybd_event.VK_C, 'D': keybd_event.VK_D,
		'E': keybd_event.VK_E, 'F': keybd_event.VK_F, 'G': keybd_event.VK_G, 'H': keybd_event.VK_H,
		'I': keybd_event.VK_I, 'J': keybd_event.VK_J, 'K': keybd_event.VK_K, 'L': keybd_event.VK_L,
		'M': keybd_event.VK_M, 'N': keybd_event.VK_N, 'O': keybd_event.VK_O, 'P': keybd_event.VK_P,
		'Q': keybd_event.VK_Q, 'R': keybd_event.VK_R, 'S': keybd_event.VK_S, 'T': keybd_event.VK_T,
		'U': keybd_event.VK_U, 'V': keybd_event.VK_V, 'W': keybd_event.VK_W, 'X': keybd_event.VK_X,
		'Y': keybd_event.VK_Y, 'Z': keybd_event.VK_Z,
	}

	// Mapping for numbers
	numberToKeyCode := map[rune]int{
		'1': keybd_event.VK_1, '2': keybd_event.VK_2, '3': keybd_event.VK_3, '4': keybd_event.VK_4,
		'5': keybd_event.VK_5, '6': keybd_event.VK_6, '7': keybd_event.VK_7, '8': keybd_event.VK_8,
		'9': keybd_event.VK_9, '0': keybd_event.VK_0,
	}

	// Mapping for special characters
	specialKeys := map[rune]int{
		' ':  keybd_event.VK_SPACE,
		'.':  keybd_event.VK_Period,
		',':  keybd_event.VK_COMMA,
		'!':  keybd_event.VK_1,
		'@':  keybd_event.VK_2,
		'#':  keybd_event.VK_3,
		'$':  keybd_event.VK_4,
		'%':  keybd_event.VK_5,
		'^':  keybd_event.VK_6,
		'&':  keybd_event.VK_7,
		'*':  keybd_event.VK_8,
		'(':  keybd_event.VK_9,
		')':  keybd_event.VK_0,
		'-':  keybd_event.VK_MINUS,
		'=':  keybd_event.VK_EQUAL,
		'[':  keybd_event.VK_LeftBracket,
		']':  keybd_event.VK_RightBracket,
		';':  keybd_event.VK_SEMICOLON,
		'\'': keybd_event.VK_Quote,
		'\\': keybd_event.VK_BACKSLASH,
		'/':  keybd_event.VK_SLASH,
		'`':  keybd_event.VK_GRAVE,
	}

	for _, char := range str {
		// Reset the KeyBonding
		kb.Clear()

		// Determine the key code and whether Shift is needed
		var keyCode int
		needsShift := false

		if keyCode, needsShift = getKeyCode(char, charToKeyCode, numberToKeyCode, specialKeys); keyCode == -1 {
			// Skip unsupported characters
			continue
		}

		// Set Shift if needed
		kb.HasSHIFT(needsShift)

		// Set the key
		kb.SetKeys(keyCode)

		// kb.Launching is slow
		kb.Press()
		kb.Release()
	}

	return nil
}

// Helper function to get the appropriate key code and shift status
func getKeyCode(char rune, charMap, numberMap, specialMap map[rune]int) (int, bool) {
	// Check uppercase letters first
	if keyCode, ok := charMap[char]; ok {
		return keyCode, unicode.IsUpper(char)
	}

	// Check numbers
	if keyCode, ok := numberMap[char]; ok {
		return keyCode, false
	}

	// Check special characters
	if keyCode, ok := specialMap[char]; ok {
		needsShift := strings.ContainsRune("!@#$%^&*()_+{}|:\"<>?", char)
		return keyCode, needsShift
	}

	// Unsupported character
	return -1, false
}
