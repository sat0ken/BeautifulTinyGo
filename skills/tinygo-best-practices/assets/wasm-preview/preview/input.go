//go:build tinygo && wasm

package preview

import "syscall/js"

// Key names follow KeyboardEvent.key in the browser: "ArrowLeft", "z", " ", ...
//
// The page calls previewKey(key, down) on every keydown and keyup, and
// tools/capture.cjs does the same for scripted key presses.

var (
	down    = map[string]bool{}
	keyFunc js.Func
)

func init() {
	keyFunc = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) < 2 {
			return nil
		}
		down[args[0].String()] = args[1].Bool()
		return nil
	})
	js.Global().Set("previewKey", keyFunc)
}

// Pressed reports whether the key is held down right now.
func Pressed(key string) bool {
	return down[key]
}

// Button returns a func() bool that is true while any of the keys is held. It
// has the same shape as a hardware button reader such as
// func() bool { return !pin.Get() }, so the application can take buttons as
// plain functions on both builds.
func Button(keys ...string) func() bool {
	return func() bool {
		for _, k := range keys {
			if down[k] {
				return true
			}
		}
		return false
	}
}
