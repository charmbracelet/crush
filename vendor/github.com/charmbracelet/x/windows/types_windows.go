package windows

import (
	"encoding/binary"

	"golang.org/x/sys/windows"
)

// Virtual Key codes
// https://docs.microsoft.com/en-us/windows/win32/inputdev/virtual-key-codes
const (
	VK_LBUTTON             = 0x01
	VK_RBUTTON             = 0x02
	VK_CANCEL              = 0x03
	VK_MBUTTON             = 0x04
	VK_XBUTTON1            = 0x05
	VK_XBUTTON2            = 0x06
	VK_BACK                = 0x08
	VK_TAB                 = 0x09
	VK_CLEAR               = 0x0C
	VK_RETURN              = 0x0D
	VK_SHIFT               = 0x10
	VK_CONTROL             = 0x11
	VK_MENU                = 0x12
	VK_PAUSE               = 0x13
	VK_CAPITAL             = 0x14
	VK_KANA                = 0x15
	VK_HANGEUL             = 0x15
	VK_HANGUL              = 0x15
	VK_IME_ON              = 0x16
	VK_JUNJA               = 0x17
	VK_FINAL               = 0x18
	VK_HANJA               = 0x19
	VK_KANJI               = 0x19
	VK_IME_OFF             = 0x1A
	VK_ESCAPE              = 0x1B
	VK_CONVERT             = 0x1C
	VK_NONCONVERT          = 0x1D
	VK_ACCEPT              = 0x1E
	VK_MODECHANGE          = 0x1F
	VK_SPACE               = 0x20
	VK_PRIOR               = 0x21
	VK_NEXT                = 0x22
	VK_END                 = 0x23
	VK_HOME                = 0x24
	VK_LEFT                = 0x25
	VK_UP                  = 0x26
	VK_RIGHT               = 0x27
	VK_DOWN                = 0x28
	VK_SELECT              = 0x29
	VK_PRINT               = 0x2A
	VK_EXECUTE             = 0x2B
	VK_SNAPSHOT            = 0x2C
	VK_INSERT              = 0x2D
	VK_DELETE              = 0x2E
	VK_HELP                = 0x2F
	VK_LWIN                = 0x5B
	VK_RWIN                = 0x5C
	VK_APPS                = 0x5D
	VK_SLEEP               = 0x5F
	VK_NUMPAD0             = 0x60
	VK_NUMPAD1             = 0x61
	VK_NUMPAD2             = 0x62
	VK_NUMPAD3             = 0x63
	VK_NUMPAD4             = 0x64
	VK_NUMPAD5             = 0x65
	VK_NUMPAD6             = 0x66
	VK_NUMPAD7             = 0x67
	VK_NUMPAD8             = 0x68
	VK_NUMPAD9             = 0x69
	VK_MULTIPLY            = 0x6A
	VK_ADD                 = 0x6B
	VK_SEPARATOR           = 0x6C
	VK_SUBTRACT            = 0x6D
	VK_DECIMAL             = 0x6E
	VK_DIVIDE              = 0x6F
	VK_F1                  = 0x70
	VK_F2                  = 0x71
	VK_F3                  = 0x72
	VK_F4                  = 0x73
	VK_F5                  = 0x74
	VK_F6                  = 0x75
	VK_F7                  = 0x76
	VK_F8                  = 0x77
	VK_F9                  = 0x78
	VK_F10                 = 0x79
	VK_F11                 = 0x7A
	VK_F12                 = 0x7B
	VK_F13                 = 0x7C
	VK_F14                 = 0x7D
	VK_F15                 = 0x7E
	VK_F16                 = 0x7F
	VK_F17                 = 0x80
	VK_F18                 = 0x81
	VK_F19                 = 0x82
	VK_F20                 = 0x83
	VK_F21                 = 0x84
	VK_F22                 = 0x85
	VK_F23                 = 0x86
	VK_F24                 = 0x87
	VK_NUMLOCK             = 0x90
	VK_SCROLL              = 0x91
	VK_OEM_NEC_EQUAL       = 0x92
	VK_OEM_FJ_JISHO        = 0x92
	VK_OEM_FJ_MASSHOU      = 0x93
	VK_OEM_FJ_TOUROKU      = 0x94
	VK_OEM_FJ_LOYA         = 0x95
	VK_OEM_FJ_ROYA         = 0x96
	VK_LSHIFT              = 0xA0
	VK_RSHIFT              = 0xA1
	VK_LCONTROL            = 0xA2
	VK_RCONTROL            = 0xA3
	VK_LMENU               = 0xA4
	VK_RMENU               = 0xA5
	VK_BROWSER_BACK        = 0xA6
	VK_BROWSER_FORWARD     = 0xA7
	VK_BROWSER_REFRESH     = 0xA8
	VK_BROWSER_STOP        = 0xA9
	VK_BROWSER_SEARCH      = 0xAA
	VK_BROWSER_FAVORITES   = 0xAB
	VK_BROWSER_HOME        = 0xAC
	VK_VOLUME_MUTE         = 0xAD
	VK_VOLUME_DOWN         = 0xAE
	VK_VOLUME_UP           = 0xAF
	VK_MEDIA_NEXT_TRACK    = 0xB0
	VK_MEDIA_PREV_TRACK    = 0xB1
	VK_MEDIA_STOP          = 0xB2
	VK_MEDIA_PLAY_PAUSE    = 0xB3
	VK_LAUNCH_MAIL         = 0xB4
	VK_LAUNCH_MEDIA_SELECT = 0xB5
	VK_LAUNCH_APP1         = 0xB6
	VK_LAUNCH_APP2         = 0xB7
	VK_OEM_1               = 0xBA
	VK_OEM_PLUS            = 0xBB
	VK_OEM_COMMA           = 0xBC
	VK_OEM_MINUS           = 0xBD
	VK_OEM_PERIOD          = 0xBE
	VK_OEM_2               = 0xBF
	VK_OEM_3               = 0xC0
	VK_OEM_4               = 0xDB
	VK_OEM_5               = 0xDC
	VK_OEM_6               = 0xDD
	VK_OEM_7               = 0xDE
	VK_OEM_8               = 0xDF
	VK_OEM_AX              = 0xE1
	VK_OEM_102             = 0xE2
	VK_ICO_HELP            = 0xE3
	VK_ICO_00              = 0xE4
	VK_PROCESSKEY          = 0xE5
	VK_ICO_CLEAR           = 0xE6
	VK_OEM_RESET           = 0xE9
	VK_OEM_JUMP            = 0xEA
	VK_OEM_PA1             = 0xEB
	VK_OEM_PA2             = 0xEC
	VK_OEM_PA3             = 0xED
	VK_OEM_WSCTRL          = 0xEE
	VK_OEM_CUSEL           = 0xEF
	VK_OEM_ATTN            = 0xF0
	VK_OEM_FINISH          = 0xF1
	VK_OEM_COPY            = 0xF2
	VK_OEM_AUTO            = 0xF3
	VK_OEM_ENLW            = 0xF4
	VK_OEM_BACKTAB         = 0xF5
	VK_ATTN                = 0xF6
	VK_CRSEL               = 0xF7
	VK_EXSEL               = 0xF8
	VK_EREOF               = 0xF9
	VK_PLAY                = 0xFA
	VK_ZOOM                = 0xFB
	VK_NONAME              = 0xFC
	VK_PA1                 = 0xFD
	VK_OEM_CLEAR           = 0xFE
)

// Mouse button constants.
// https://docs.microsoft.com/en-us/windows/console/mouse-event-record-str
const (
	FROM_LEFT_1ST_BUTTON_PRESSED = 0x0001
	RIGHTMOST_BUTTON_PRESSED     = 0x0002
	FROM_LEFT_2ND_BUTTON_PRESSED = 0x0004
	FROM_LEFT_3RD_BUTTON_PRESSED = 0x0008
	FROM_LEFT_4TH_BUTTON_PRESSED = 0x0010
)

// Control key state constaints.
// https://docs.microsoft.com/en-us/windows/console/key-event-record-str
// https://docs.microsoft.com/en-us/windows/console/mouse-event-record-str
const (
	CAPSLOCK_ON        = 0x0080
	ENHANCED_KEY       = 0x0100
	LEFT_ALT_PRESSED   = 0x0002
	LEFT_CTRL_PRESSED  = 0x0008
	NUMLOCK_ON         = 0x0020
	RIGHT_ALT_PRESSED  = 0x0001
	RIGHT_CTRL_PRESSED = 0x0004
	SCROLLLOCK_ON      = 0x0040
	SHIFT_PRESSED      = 0x0010
)

// Mouse event record event flags.
// https://docs.microsoft.com/en-us/windows/console/mouse-event-record-str
const (
	MOUSE_MOVED    = 0x0001
	DOUBLE_CLICK   = 0x0002
	MOUSE_WHEELED  = 0x0004
	MOUSE_HWHEELED = 0x0008
)

// Input Record Event Types
// https://learn.microsoft.com/en-us/windows/console/input-record-str
const (
	FOCUS_EVENT              = 0x0010
	KEY_EVENT                = 0x0001
	MENU_EVENT               = 0x0008
	MOUSE_EVENT              = 0x0002
	WINDOW_BUFFER_SIZE_EVENT = 0x0004
)

// FocusEventRecord corresponds to the FocusEventRecord structure from the
// Windows console API.
// https://docs.microsoft.com/en-us/windows/console/focus-event-record-str
type FocusEventRecord struct {
	// SetFocus is reserved and should not be used.
	SetFocus bool
}

// KeyEventRecord corresponds to the KeyEventRecord structure from the Windows
// console API.
// https://docs.microsoft.com/en-us/windows/console/key-event-record-str
type KeyEventRecord struct {
	// KeyDown specified whether the key is pressed or released.
	KeyDown bool

	//  RepeatCount indicates that a key is being held down. For example, when a
	//  key is held down, five events with RepeatCount equal to 1 may be
	//  generated, one event with RepeatCount equal to 5, or multiple events
	//  with RepeatCount greater than or equal to 1.
	RepeatCount uint16

	// VirtualKeyCode identifies the given key in a device-independent manner
	// (see
	// https://docs.microsoft.com/en-us/windows/win32/inputdev/virtual-key-codes).
	VirtualKeyCode uint16

	//  VirtualScanCode represents the device-dependent value generated by the
	//  keyboard hardware.
	VirtualScanCode uint16

	// Char is the character that corresponds to the pressed key. Char can be
	// zero for some keys.
	Char rune

	// ControlKeyState holds the state of the control keys.
	ControlKeyState uint32
}

// MenuEventRecord corresponds to the MenuEventRecord structure from the
// Windows console API.
// https://docs.microsoft.com/en-us/windows/console/menu-event-record-str
type MenuEventRecord struct {
	CommandID uint32
}

// MouseEventRecord corresponds to the MouseEventRecord structure from the
// Windows console API.
// https://docs.microsoft.com/en-us/windows/console/mouse-event-record-str
type MouseEventRecord struct {
	// MousePosition contains the location of the cursor, in terms of the
	// console screen buffer's character-cell coordinates.
	MousePositon windows.Coord

	// ButtonState holds the status of the mouse buttons.
	ButtonState uint32

	// ControlKeyState holds the state of the control keys.
	ControlKeyState uint32

	// EventFlags specify the type of mouse event.
	EventFlags uint32
}

// WindowBufferSizeRecord corresponds to the WindowBufferSizeRecord structure
// from the Windows console API.
// https://docs.microsoft.com/en-us/windows/console/window-buffer-size-record-str
type WindowBufferSizeRecord struct {
	// Size contains the size of the console screen buffer, in character cell
	// columns and rows.
	Size windows.Coord
}

// InputRecord corresponds to the INPUT_RECORD structure from the Windows
// console API.
//
// https://docs.microsoft.com/en-us/windows/console/input-record-str
type InputRecord struct {
	// EventType specifies the type of event that helt in Event.
	EventType uint16

	// Padding of the 16-bit EventType to a whole 32-bit dword.
	_ [2]byte

	// Event holds the actual event data.
	Event [16]byte
}

// FocusEvent returns the event as a FOCUS_EVENT_RECORD.
func (ir InputRecord) FocusEvent() FocusEventRecord {
	return FocusEventRecord{SetFocus: ir.Event[0] > 0}
}

// KeyEvent returns the event as a KEY_EVENT_RECORD.
func (ir InputRecord) KeyEvent() KeyEventRecord {
	return KeyEventRecord{
		KeyDown:         binary.LittleEndian.Uint32(ir.Event[0:4]) > 0,
		RepeatCount:     binary.LittleEndian.Uint16(ir.Event[4:6]),
		VirtualKeyCode:  binary.LittleEndian.Uint16(ir.Event[6:8]),
		VirtualScanCode: binary.LittleEndian.Uint16(ir.Event[8:10]),
		Char:            rune(binary.LittleEndian.Uint16(ir.Event[10:12])),
		ControlKeyState: binary.LittleEndian.Uint32(ir.Event[12:16]),
	}
}

// MouseEvent returns the event as a MOUSE_EVENT_RECORD.
func (ir InputRecord) MouseEvent() MouseEventRecord {
	return MouseEventRecord{
		MousePositon: windows.Coord{
			X: int16(binary.LittleEndian.Uint16(ir.Event[0:2])),
			Y: int16(binary.LittleEndian.Uint16(ir.Event[2:4])),
		},
		ButtonState:     binary.LittleEndian.Uint32(ir.Event[4:8]),
		ControlKeyState: binary.LittleEndian.Uint32(ir.Event[8:12]),
		EventFlags:      binary.LittleEndian.Uint32(ir.Event[12:16]),
	}
}

// WindowBufferSizeEvent returns the event as a WINDOW_BUFFER_SIZE_RECORD.
func (ir InputRecord) WindowBufferSizeEvent() WindowBufferSizeRecord {
	return WindowBufferSizeRecord{
		Size: windows.Coord{
			X: int16(binary.LittleEndian.Uint16(ir.Event[0:2])),
			Y: int16(binary.LittleEndian.Uint16(ir.Event[2:4])),
		},
	}
}

// MenuEvent returns the event as a MENU_EVENT_RECORD.
func (ir InputRecord) MenuEvent() MenuEventRecord {
	return MenuEventRecord{
		CommandID: binary.LittleEndian.Uint32(ir.Event[0:4]),
	}
}
