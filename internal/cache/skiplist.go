package cache

import (
	"math/rand"
	"sync"
	"time"
)

const (
	MaxLevel = 128
	P        = 0.25
)

type SkipListNode[K, V comparable] struct {
	Key     K
	Value   V
	Forward []*SkipListNode[K, V]
}

type CompareFunc[V comparable] func(a, b V) int

type SkipList[K, V comparable] struct {
	mu     sync.RWMutex
	length int
	level  int
	rand   *rand.Rand

	header   *SkipListNode[K, V]
	mapIndex map[K]*SkipListNode[K, V]
	compare  CompareFunc[V]
}

type Entry[K comparable, V comparable] struct {
	Key   K
	Value V
	Rank  int
}

func NewSkipList[K, V comparable](compareFunc CompareFunc[V]) *SkipList[K, V] {
	header := &SkipListNode[K, V]{
		Forward: make([]*SkipListNode[K, V], MaxLevel),
	}

	return &SkipList[K, V]{
		header: header,
		level:  1,
		// keyIndex:  make(map[K]*SkipListNode[K, V]),
		mapIndex: make(map[K]*SkipListNode[K, V]),
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
		compare:  compareFunc,
	}
}

func (sl *SkipList[K, V]) randomLevel() int {
	level := 1
	for level < MaxLevel && sl.rand.Float64() < P {
		level++
	}
	return level
}

// InsertOrUpdate inserts a new score or updates existing node's value
func (sl *SkipList[K, V]) InsertOrUpdate(key K, value V) bool {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	// Check if node already exists
	if existingNode, nodeExists := sl.mapIndex[key]; nodeExists {
		// Node exists, check if new score is better
		if sl.compare(value, existingNode.Value) < 0 {
			// New score is better, remove old entry and add new one
			sl.deleteNode(existingNode.Key, existingNode.Value)
			return sl.insertNode(key, value)
		}
		// Existing score is better or equal, don't update
		return false
	}

	// User doesn't exist, insert new entry
	return sl.insertNode(key, value)
}

// insertNode is the internal method to insert a node
func (sl *SkipList[K, V]) insertNode(key K, value V) bool {
	update := make([]*SkipListNode[K, V], MaxLevel)
	x := sl.header

	for i := sl.level - 1; i >= 0; i-- {
		for x.Forward[i] != nil && sl.compare(x.Forward[i].Value, value) < 0 {
			x = x.Forward[i]
		}
		update[i] = x
	}

	newLevel := sl.randomLevel()
	if newLevel > sl.level {
		for i := sl.level; i < newLevel; i++ {
			update[i] = sl.header
		}
		sl.level = newLevel
	}

	newNode := &SkipListNode[K, V]{
		Key:     key,
		Value:   value,
		Forward: make([]*SkipListNode[K, V], newLevel),
	}

	for i := range newLevel {
		newNode.Forward[i] = update[i].Forward[i]
		update[i].Forward[i] = newNode
	}

	// sl.keyIndex[key] = newNode
	sl.mapIndex[key] = newNode
	sl.length++
	return true
}

// Delete removes a node by key
func (sl *SkipList[K, V]) Delete(key K) bool {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	node, exists := sl.mapIndex[key]
	if !exists {
		return false
	}
	return sl.deleteNode(node.Key, node.Value)
}

// deleteNode is the internal method to delete a node
func (sl *SkipList[K, V]) deleteNode(key K, value V) bool {
	update := make([]*SkipListNode[K, V], MaxLevel)
	x := sl.header

	for i := sl.level - 1; i >= 0; i-- {
		for x.Forward[i] != nil && sl.compare(x.Forward[i].Value, value) < 0 {
			x = x.Forward[i]
		}
		update[i] = x
	}

	x = x.Forward[0]
	if x != nil && sl.compare(x.Value, value) == 0 {
		for i := 0; i < sl.level; i++ {
			if update[i].Forward[i] != x {
				break
			}
			update[i].Forward[i] = x.Forward[i]
		}

		for sl.level > 1 && sl.header.Forward[sl.level-1] == nil {
			sl.level--
		}

		// delete(sl.keyIndex, key)
		delete(sl.mapIndex, key)
		sl.length--
		return true
	}
	return false
}

func (sl *SkipList[K, V]) Search(key K) (V, bool) {
	// sl.mu.RLock()
	// defer sl.mu.RUnlock()

	node, exists := sl.mapIndex[key]
	if !exists {
		var zero V
		return zero, false
	}
	return node.Value, true
}

func (sl *SkipList[K, V]) GetRank(key K) (int, bool) {
	// sl.mu.RLock()
	// defer sl.mu.RUnlock()

	node, exists := sl.mapIndex[key]
	if !exists {
		return 0, false
	}

	rank := 1
	x := sl.header

	for i := sl.level - 1; i >= 0; i-- {
		for x.Forward[i] != nil && sl.compare(x.Forward[i].Value, node.Value) < 0 {
			rank++
			x = x.Forward[i]
		}
	}

	return rank, true
}

func (sl *SkipList[K, V]) GetTopK(k int) []Entry[K, V] {
	// sl.mu.RLock()
	// defer sl.mu.RUnlock()

	result := make([]Entry[K, V], 0, k)
	x := sl.header.Forward[0]

	for i := 0; i < k && x != nil; i++ {
		result = append(result, Entry[K, V]{
			Key:   x.Key,
			Value: x.Value,
			Rank:  i + 1,
		})
		x = x.Forward[0]
	}

	return result
}

func (sl *SkipList[K, V]) GetAll() []Entry[K, V] {
	// sl.mu.RLock()
	// defer sl.mu.RUnlock()

	result := make([]Entry[K, V], 0, sl.length)
	x := sl.header.Forward[0]
	rank := 1

	for x != nil {
		result = append(result, Entry[K, V]{
			Key:   x.Key,
			Value: x.Value,
			Rank:  rank,
		})
		x = x.Forward[0]
		rank++
	}

	return result
}

// GetAllExpiredEntries returns entries older than the cutoff time
func (sl *SkipList[K, V]) GetAllExpiredEntries(isExpired func(K) bool) []Entry[K, V] {
	// sl.mu.RLock()
	// defer sl.mu.RUnlock()

	result := make([]Entry[K, V], 0)
	x := sl.header.Forward[0]
	rank := 1

	for x != nil {
		if isExpired(x.Key) {
			result = append(result, Entry[K, V]{
				Key:   x.Key,
				Value: x.Value,
				Rank:  rank,
			})
		}
		x = x.Forward[0]
		rank++
	}

	return result
}

func (sl *SkipList[K, V]) GetLength() int {
	// sl.mu.RLock()
	// defer sl.mu.RUnlock()
	return sl.length
}

func (sl *SkipList[K, V]) Contains(key K) bool {
	// sl.mu.RLock()
	// defer sl.mu.RUnlock()
	_, exists := sl.mapIndex[key]
	return exists
}

func (sl *SkipList[K, V]) IsEmpty() bool {
	// sl.mu.RLock()
	// defer sl.mu.RUnlock()
	return sl.length == 0
}

func (sl *SkipList[K, V]) Clear() {
	// sl.mu.Lock()
	// defer sl.mu.Unlock()

	sl.header = &SkipListNode[K, V]{
		Forward: make([]*SkipListNode[K, V], MaxLevel),
	}
	sl.level = 1
	sl.length = 0
	sl.mapIndex = make(map[K]*SkipListNode[K, V])
}
