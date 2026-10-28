package agent

import (
	"hash/fnv"
	"strings"
)

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

// SubagentName gives a subagent its name: the label it was dispatched with,
// prefixed with a first name chosen from the label, so "date-impl" becomes
// "betty-date-impl". The dispatch result hands that name back and everything
// after addresses it by name. A name passes through unchanged, so the model
// can use either form.
func SubagentName(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	for _, first := range subagentFirstNames {
		if strings.HasPrefix(label, first+"-") {
			return label
		}
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(label))
	return subagentFirstNames[h.Sum32()%uint32(len(subagentFirstNames))] + "-" + label
}
