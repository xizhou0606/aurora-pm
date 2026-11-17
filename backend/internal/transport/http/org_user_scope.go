package httpapi

import (
	"sort"
)

const pmUserFactIDSQL = "users.id"

func mergeSortedUserIDs(ids []uint, extra ...uint) []uint {
	seen := map[uint]struct{}{}
	for _, id := range append(ids, extra...) {
		if id > 0 {
			seen[id] = struct{}{}
		}
	}
	ids = make([]uint, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
