package chat

import "hash/fnv"

// subagentFirstNames give each sub-agent a person's name in the UI, borrowed
// from charmbracelet/creeper.
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

// SubagentDisplayName prefixes a sub-agent's label with a first name, so
// "date-impl" reads as "betty-date-impl". The name only ever reaches the
// screen: the model keeps addressing the sub-agent by the label it chose,
// so it never sees a name it did not ask for. The name is picked from the
// label alone, so every place that shows the sub-agent (the dispatch, its
// report, a follow-up, the sidebar) agrees without sharing any state, and
// it survives a restart.
func SubagentDisplayName(label string) string {
	if label == "" {
		return ""
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(label))
	return subagentFirstNames[h.Sum32()%uint32(len(subagentFirstNames))] + "-" + label
}
