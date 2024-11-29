package main

import (
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/micmonay/keybd_event"
)

// Explicit mapping for a-z characters
var charToKeyCode = map[rune]int{
	'a': keybd_event.VK_A, 'b': keybd_event.VK_B, 'c': keybd_event.VK_C, 'd': keybd_event.VK_D,
	'e': keybd_event.VK_E, 'f': keybd_event.VK_F, 'g': keybd_event.VK_G, 'h': keybd_event.VK_H,
	'i': keybd_event.VK_I, 'j': keybd_event.VK_J, 'k': keybd_event.VK_K, 'l': keybd_event.VK_L,
	'm': keybd_event.VK_M, 'n': keybd_event.VK_N, 'o': keybd_event.VK_O, 'p': keybd_event.VK_P,
	'q': keybd_event.VK_Q, 'r': keybd_event.VK_R, 's': keybd_event.VK_S, 't': keybd_event.VK_T,
	'u': keybd_event.VK_U, 'v': keybd_event.VK_V, 'w': keybd_event.VK_W, 'x': keybd_event.VK_X,
	'y': keybd_event.VK_Y, 'z': keybd_event.VK_Z,
}

// Mapping for numbers
var numberToKeyCode = map[rune]int{
	'1': keybd_event.VK_1, '2': keybd_event.VK_2, '3': keybd_event.VK_3, '4': keybd_event.VK_4,
	'5': keybd_event.VK_5, '6': keybd_event.VK_6, '7': keybd_event.VK_7, '8': keybd_event.VK_8,
	'9': keybd_event.VK_9, '0': keybd_event.VK_0,
}

// Mapping for special characters
var specialKeys = map[rune]int{
	' ':  keybd_event.VK_SPACE,
	'.':  keybd_event.VK_Period,
	',':  keybd_event.VK_COMMA,
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

var shiftedSpecialKeys = map[rune]int{
	'~': keybd_event.VK_GRAVE,
	'!': keybd_event.VK_1,
	'@': keybd_event.VK_2,
	'#': keybd_event.VK_3,
	'$': keybd_event.VK_4,
	'%': keybd_event.VK_5,
	'^': keybd_event.VK_6,
	'&': keybd_event.VK_7,
	'*': keybd_event.VK_8,
	'(': keybd_event.VK_9,
	')': keybd_event.VK_0,
	'_': keybd_event.VK_MINUS,
	'+': keybd_event.VK_EQUAL,
	'{': keybd_event.VK_LeftBracket,
	'}': keybd_event.VK_RightBracket,
	':': keybd_event.VK_SEMICOLON,
	'"': keybd_event.VK_Quote,
	'|': keybd_event.VK_BACKSLASH,
	'>': keybd_event.VK_Period,
	'<': keybd_event.VK_COMMA,
	'?': keybd_event.VK_SLASH,
}

func typeString(str string, kb keybd_event.KeyBonding) error {
	for _, char := range str {
		// Reset the KeyBonding
		kb.Clear()

		// Determine the key code and whether Shift is needed
		var keyCode int
		needsShift := false

		keyCode, needsShift = getKeyCode(char)
		if keyCode == -1 {
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
func getKeyCode(char rune) (int, bool) {
	// Check uppercase letters first
	if keyCode, ok := charToKeyCode[unicode.ToLower(char)]; ok {
		return keyCode, unicode.IsUpper(char)
	}

	// Check numbers
	if keyCode, ok := numberToKeyCode[char]; ok {
		return keyCode, false
	}

	// Check special characters
	if keyCode, ok := specialKeys[char]; ok {
		return keyCode, false
	}

	if keyCode, ok := shiftedSpecialKeys[char]; ok {
		return keyCode, true
	}

	// Unsupported character
	return -1, false
}

// pasteString is an alternative to typing the string. Typing uses
// actual keystrokes which could cause issue if we are not in a text
// field or if we have random control keys pressed down.
func pasteString(str string, kb keybd_event.KeyBonding) error {
	// Store the current clipboard content
	currentContent, err := clipboard.ReadAll()
	if err != nil {
		return err
	}

	// Write the new string to the clipboard
	err = clipboard.WriteAll(str)
	if err != nil {
		return err
	}

	// Paste the new string
	kb.SetKeys(keybd_event.VK_V)
	kb.HasSuper(true)
	err = kb.Launching()
	if err != nil {
		return err
	}

	// Restore the original clipboard content
	// Looks like if you do this fast enough clipboard managers don't get to store it
	err = clipboard.WriteAll(currentContent)
	if err != nil {
		return err
	}

	return nil
}
