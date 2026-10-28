package agent

import (
	"math/rand/v2"
	"strings"
	"sync"
)

// subagentFirstName pairs the ASCII key a model types with the accented
// form a person reads.
type subagentFirstName struct {
	key     string
	display string
}

// subagentFirstNames are borrowed from charmbracelet/creeper.
var subagentFirstNames = []subagentFirstName{
	{"akiko", "Akiko"},
	{"alejandra", "Alejandra"},
	{"alexander", "Alexander"},
	{"athena", "Athena"},
	{"benedict", "Benedict"},
	{"benoit", "Benoît"},
	{"betty", "Betty"},
	{"bjorn", "Björn"},
	{"candy", "Candy"},
	{"cindy", "Cindy"},
	{"constance", "Constance"},
	{"domina", "Domina"},
	{"emily", "Emily"},
	{"francis", "Francis"},
	{"freddy", "Freddy"},
	{"gunnar", "Gunnar"},
	{"jacopo", "Jacopo"},
	{"linda", "Linda"},
	{"luka", "Luka"},
	{"madeline", "Madeline"},
	{"massimo", "Massimo"},
	{"mindy", "Mindy"},
	{"sarah", "Sarah"},
	{"sasha", "Sasha"},
	{"sofia", "Sofía"},
	{"xochitl", "Xóchitl"},
}

// subagentNamer cycles through one shuffle of the first names, so a name
// never repeats until every other one has been used.
type subagentNamer struct {
	once  sync.Once
	mu    sync.Mutex
	order []int
	next  int
}

// name puts the next first name in front of label: "date-impl" becomes
// something like "betty-date-impl". A label that already has one keeps it.
func (n *subagentNamer) name(label string) string {
	label = strings.TrimSpace(label)
	if _, _, ok := splitSubagentName(label); ok || label == "" {
		return label
	}
	n.once.Do(func() { n.order = rand.Perm(len(subagentFirstNames)) })
	n.mu.Lock()
	first := subagentFirstNames[n.order[n.next%len(n.order)]]
	n.next++
	n.mu.Unlock()
	return first.key + "-" + label
}

func splitSubagentName(name string) (subagentFirstName, string, bool) {
	for _, first := range subagentFirstNames {
		if rest, ok := strings.CutPrefix(name, first.key+"-"); ok && rest != "" {
			return first, rest, true
		}
	}
	return subagentFirstName{}, name, false
}

// SubagentDisplayName splits a subagent's name into its accented first name
// and its label. A label without a first name comes back with first empty.
func SubagentDisplayName(name string) (first, label string) {
	f, label, _ := splitSubagentName(name)
	return f.display, label
}
