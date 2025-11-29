package message

import (
	"encoding/json"
	"fmt"

	"github.com/alpkeskin/gotoon"
)

func ConvertJSONToTOON(text string) string {
	if text == "" {
		return text
	}

	var data interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		fmt.Println("[TOON] Not JSON, returning original:", text[:min(50, len(text))])
		return text
	}

	toonText, err := gotoon.Encode(data)
	if err != nil {
		fmt.Println("[TOON] Encoding error:", err)
		return text
	}

	fmt.Println("[TOON] Converted JSON to TOON")
	fmt.Println("[TOON] Original:", text[:min(100, len(text))])
	fmt.Println("[TOON] Result:", toonText[:min(100, len(toonText))])
	return toonText
}
