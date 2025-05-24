package cache

import (
	"math/rand"
	"sync"
	"time"
)

const (
	MaxLevel = 16
	P        = 0.5
)

type SkipListNode[K comparable, V any] struct {
	Key     K
	Value   V
	Forward []*SkipListNode[K, V]
}

type CompareFunc[K comparable] func(a, b K) int

type SkipList[K comparable, V any] struct {
	mu       sync.RWMutex
	length   int
	header   *SkipListNode[K, V]
	level    int
	keyIndex map[K]*SkipListNode[K, V]
	rand     *rand.Rand
	compare  CompareFunc[K]
}

func NewSkipList[K comparable, V any](compareFunc CompareFunc[K]) *SkipList[K, V] {
	header := &SkipListNode[K, V]{
		Forward: make([]*SkipListNode[K, V], MaxLevel),
	}

	return &SkipList[K, V]{
		header:   header,
		level:    1,
		keyIndex: make(map[K]*SkipListNode[K, V]),
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

func (sl *SkipList[K, V]) Insert(key K, value V) bool {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if existing, exists := sl.keyIndex[key]; exists {
		existing.Value = value
		return false
	}

	update := make([]*SkipListNode[K, V], MaxLevel)
	x := sl.header

	for i := sl.level - 1; i >= 0; i-- {
		for x.Forward[i] != nil && sl.compare(x.Forward[i].Key, key) < 0 {
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

	for i := 0; i < newLevel; i++ {
		newNode.Forward[i] = update[i].Forward[i]
		update[i].Forward[i] = newNode
	}

	sl.keyIndex[key] = newNode
	sl.length++
	return true
}

func (sl *SkipList[K, V]) Delete(key K) bool {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	update := make([]*SkipListNode[K, V], MaxLevel)
	x := sl.header

	for i := sl.level - 1; i >= 0; i-- {
		for x.Forward[i] != nil && sl.compare(x.Forward[i].Key, key) < 0 {
			x = x.Forward[i]
		}
		update[i] = x
	}

	x = x.Forward[0]
	if x != nil && sl.compare(x.Key, key) == 0 {
		for i := 0; i < sl.level; i++ {
			if update[i].Forward[i] != x {
				break
			}
			update[i].Forward[i] = x.Forward[i]
		}

		for sl.level > 1 && sl.header.Forward[sl.level-1] == nil {
			sl.level--
		}

		delete(sl.keyIndex, key)
		sl.length--
		return true
	}
	return false
}

func (sl *SkipList[K, V]) Search(key K) (V, bool) {
	// sl.mu.RLock()
	// defer sl.mu.RUnlock()

	node, exists := sl.keyIndex[key]
	if !exists {
		var zero V
		return zero, false
	}
	return node.Value, true
}

func (sl *SkipList[K, V]) GetRank(key K) (int, bool) {
	// sl.mu.RLock()
	// defer sl.mu.RUnlock()

	node, exists := sl.keyIndex[key]
	if !exists {
		return 0, false
	}

	rank := 1
	x := sl.header

	for i := sl.level - 1; i >= 0; i-- {
		for x.Forward[i] != nil && sl.compare(x.Forward[i].Key, node.Key) < 0 {
			rank++
			x = x.Forward[i]
		}
	}

	return rank, true
}

type Entry[K comparable, V any] struct {
	Key   K
	Value V
	Rank  int
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

func (sl *SkipList[K, V]) GetLength() int {
	// sl.mu.RLock()
	// defer sl.mu.RUnlock()
	return sl.length
}

func (sl *SkipList[K, V]) Contains(key K) bool {
	// sl.mu.RLock()
	// defer sl.mu.RUnlock()
	_, exists := sl.keyIndex[key]
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
	sl.keyIndex = make(map[K]*SkipListNode[K, V])
}
