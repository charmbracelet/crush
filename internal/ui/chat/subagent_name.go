package chat

import "hash/fnv"

// subagentFirstNames are borrowed from charmbracelet/creeper.
var subagentFirstNames = []string{
	"akiko",
	"alejandra",
	"alexander",
	"athena",
	"benedict",
	"benoit",
	"betty",
	"bjorn",
	"candy",
	"cindy",
	"constance",
	"domina",
	"emily",
	"francis",
	"freddy",
	"gunnar",
	"jacopo",
	"linda",
	"luka",
	"madeline",
	"massimo",
	"mindy",
	"sarah",
	"sasha",
	"sofia",
	"xochitl",
}

// SubagentDisplayName prefixes a label with a first name chosen from the label
// alone, so every view agrees. It is display-only; the model never sees it.
func SubagentDisplayName(label string) string {
	if label == "" {
		return ""
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(label))
	return subagentFirstNames[h.Sum32()%uint32(len(subagentFirstNames))] + "-" + label
}
