package jsonschema

import "sync"

const maxPooledUniqueItems = 1024

var uniqueItemsPool = sync.Pool{
	New: func() any {
		hashes := make(map[uint64][]int)
		return &hashes
	},
}

func releaseUniqueItems(hashes *map[uint64][]int) {
	clear(*hashes)
	uniqueItemsPool.Put(hashes)
}
