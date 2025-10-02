package tools

// ASCII "emoji" library for consistent terminal output
// Cliffy's personality expressed through text art
const (
	// Cliffy - the main character
	AsciiCliffy = "ᕕ( ᐛ )ᕗ"

	// Cliffy's moods and gestures
	AsciiCliffyWaving  = "( ´◔ ω◔`) ノシ"
	AsciiCliffyProud   = "(ﾉ☉ヮ⚆)ﾉ ⌒*:･ﾟ✧"
	AsciiCliffyContent = "(*・‿・)ノ⌒*:･ﾟ✧"

	// Cliffy's props - tennis ball for the ballboy
	AsciiTennisBall = `   ,odOO"bo,
 ,dOOOP'dOOOb,
,O3OP'dOO3OO33,
P",ad33O333O3Ob
?833O338333P",d
` + "`" + `88383838P,d38'
 ` + "`" + `Y8888P,d88P'
   ` + "`" + `"?8,8P"'`

	// Cliffy's tennis racket
	AsciiTennisRacket = `⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿
⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠟⠋⣉⣀⣤⣤⣤⣈⡉⠻⣿⣿⣿⣿
⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠟⢁⣴⣿⣿⣿⣿⣿⣿⣿⣿⣦⡈⢻⣿⣿
⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠃⣰⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣧⠈⣿⣿
⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠃⣰⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀⣿⣿
⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡏⢠⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡇⢠⣿⣿
⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠇⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠟⢀⣾⣿⣿
⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠀⠸⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠟⢁⣴⣿⣿⣿⣿
⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠃⣴⣦⡙⠻⠿⠿⠿⠿⠛⠋⣁⣠⣶⣿⣿⣿⣿⣿⣿
⣿⣿⣿⣿⣿⣿⣿⣿⣿⡟⠀⠛⠛⣁⣠⣤⣴⣶⣶⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿
⣿⣿⣿⣿⣿⣿⣿⡿⠋⣻⣶⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿
⣿⣿⣿⣿⣿⡿⠋⣠⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠟⠛⢿⣿⣿⣿⣿⣿⣿⣿
⣿⣿⣿⡿⠋⣠⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀⠀⠀⠀⣿⣿⣿⣿⣿⣿⣿
⣿⣿⡁⠀⣾⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣷⣤⣤⣶⣿⣿⣿⣿⣿⣿⣿
⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿`

	// Volley progress icons
	// Tennis racket head for volley start/finish
	AsciiTennisRacketHead = "◍"

	// Task status icons
	AsciiTaskComplete = "●" // Solid dot - task complete
	AsciiTaskQueued   = "○" // Hollow dot - task queued
	AsciiTaskSpinner0 = "◴" // Spinner frame 1
	AsciiTaskSpinner1 = "◵" // Spinner frame 2
	AsciiTaskSpinner2 = "◶" // Spinner frame 3
	AsciiTaskSpinner3 = "◷" // Spinner frame 4

	// Tool execution icons
	AsciiToolSuccess = "▣" // Tool succeeded
	AsciiToolFailed  = "☒" // Tool failed

	// Tree structure characters
	AsciiTreeBranch = "╮"  // Task with tools branches down
	AsciiTreeMid    = "├"  // Middle tool in list
	AsciiTreeLast   = "╰"  // Last tool in list
	AsciiTreeLine   = "───" // Horizontal connector

	// Other ASCII emojis for future use
	AsciiCheck = "[✓]"
	AsciiCross = "[✗]"
	AsciiArrow = "=>"
	AsciiDot   = "•"
)
