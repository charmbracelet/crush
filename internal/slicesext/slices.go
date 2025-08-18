package slicesext

func IsSubset[T comparable](a, b []T) bool {
	if len(a) > len(b) {
		return false
	}
	set := make(map[T]struct{}, len(b))
	for _, item := range b {
		set[item] = struct{}{}
	}
	for _, item := range a {
		if _, exists := set[item]; !exists {
			return false
		}
	}
	return true
}
