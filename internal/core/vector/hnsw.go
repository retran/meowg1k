// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package vector provides services for vector index operations including search and index building.
package vector

// hnsw.go implements a minimal pure-Go Hierarchical Navigable Small World graph for
// approximate nearest-neighbour search over float32 vectors.
//
// API surface matches the subset of github.com/coder/hnsw used by this package:
//
//	graph := NewGraph[K]()
//	graph.Add(MakeNode(key, vec))
//	graph.Export(w)  / graph.Import(r)
//	nodes := graph.Search(queryVec, topK)
//
// The implementation uses cosine distance and serialises with encoding/binary so it
// is portable across all platforms (no OS-level atomic-rename dependency).

import (
	"cmp"
	"container/heap"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"sync"
)

// ---- public types -----------------------------------------------------------

// Node holds a key and its float32 embedding.
type Node[K cmp.Ordered] struct {
	Key   K
	Value []float32
}

// MakeNode is a convenience constructor matching coder/hnsw's API.
func MakeNode[K cmp.Ordered](key K, vec []float32) Node[K] {
	return Node[K]{Key: key, Value: vec}
}

// Graph is an HNSW approximate nearest-neighbour index.
// Fields are ordered for optimal memory alignment.
type Graph[K cmp.Ordered] struct {
	nodes []hnswNode[K]
	mu    sync.RWMutex
	entry int     // index of entry-point node (-1 when empty)
	m     int     // max connections per node per layer (except layer 0 which uses 2*m)
	efC   int     // ef during construction
	maxL  int     // current highest layer
	ml    float64 // level multiplier  1/ln(m)
}

// NewGraph creates an empty HNSW graph with sensible defaults.
func NewGraph[K cmp.Ordered]() *Graph[K] {
	m := 16
	return &Graph[K]{
		entry: -1,
		m:     m,
		efC:   100,
		ml:    1.0 / math.Log(float64(m)),
	}
}

// Add inserts a node into the graph.
func (g *Graph[K]) Add(n Node[K]) {
	g.mu.Lock()
	defer g.mu.Unlock()

	idx := len(g.nodes)
	level := g.randomLevel()
	hn := hnswNode[K]{
		key:   n.Key,
		vec:   n.Value,
		level: level,
		conns: make([][]int, level+1),
	}
	for l := 0; l <= level; l++ {
		hn.conns[l] = []int{}
	}
	g.nodes = append(g.nodes, hn)

	if g.entry == -1 {
		g.entry = idx
		g.maxL = level
		return
	}

	ep := g.entry
	// Walk down from top layer to level+1
	for l := g.maxL; l > level; l-- {
		ep = g.greedyClosest(ep, n.Value, l)
	}

	// Insert layer by layer from min(level, maxL) down to 0
	for l := min(level, g.maxL); l >= 0; l-- {
		candidates := g.searchLayer(ep, n.Value, g.efC, l)
		m := g.m
		if l == 0 {
			m = g.m * 2
		}
		neighbours := g.selectNeighbours(candidates, m)

		g.nodes[idx].conns[l] = neighbours

		for _, nb := range neighbours {
			g.nodes[nb].conns[l] = append(g.nodes[nb].conns[l], idx)
			if len(g.nodes[nb].conns[l]) > m {
				// Prune: keep closest m neighbours
				kept := g.selectNeighbours(g.distHeapFromList(nb, g.nodes[nb].conns[l], l), m)
				g.nodes[nb].conns[l] = kept
			}
		}

		if len(candidates) > 0 {
			ep = candidates[0].idx
		}
	}

	if level > g.maxL {
		g.maxL = level
		g.entry = idx
	}
}

// Search returns the top-k nearest neighbours to queryVec.
func (g *Graph[K]) Search(queryVec []float32, k int) []Node[K] {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.entry == -1 || k <= 0 {
		return nil
	}

	efSearch := max(k, g.efC/4)
	if efSearch < 10 {
		efSearch = 10
	}

	ep := g.entry
	for l := g.maxL; l > 0; l-- {
		ep = g.greedyClosest(ep, queryVec, l)
	}

	candidates := g.searchLayer(ep, queryVec, efSearch, 0)

	result := make([]Node[K], 0, k)
	for i, c := range candidates {
		if i >= k {
			break
		}
		result = append(result, Node[K]{
			Key:   g.nodes[c.idx].key,
			Value: g.nodes[c.idx].vec,
		})
	}
	return result
}

// ---- serialisation ----------------------------------------------------------

const hnswMagic uint32 = 0x484E5357 // "HNSW"
const hnswVersion uint8 = 1

// Export writes the graph to w in a portable binary format.
func (g *Graph[K]) Export(w io.Writer) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Header
	if err := binary.Write(w, binary.LittleEndian, hnswMagic); err != nil {
		return fmt.Errorf("hnsw export: magic: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, hnswVersion); err != nil {
		return fmt.Errorf("hnsw export: version: %w", err)
	}
	// G115: int→int32 conversions are safe — values are graph parameters bounded to
	// small practical limits (m≤64, efC≤1000, entry/maxL≤node count).
	if err := binary.Write(w, binary.LittleEndian, int32(g.m)); err != nil { //nolint:gosec // bounded graph parameter
		return fmt.Errorf("hnsw export: m: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, int32(g.efC)); err != nil { //nolint:gosec // bounded graph parameter
		return fmt.Errorf("hnsw export: efC: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, g.ml); err != nil {
		return fmt.Errorf("hnsw export: ml: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, int32(g.entry)); err != nil { //nolint:gosec // bounded by node count
		return fmt.Errorf("hnsw export: entry: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, int32(g.maxL)); err != nil { //nolint:gosec // bounded by node count
		return fmt.Errorf("hnsw export: maxL: %w", err)
	}

	// Nodes
	if err := binary.Write(w, binary.LittleEndian, int32(len(g.nodes))); err != nil { //nolint:gosec // bounded by node count
		return fmt.Errorf("hnsw export: node count: %w", err)
	}
	for i, n := range g.nodes {
		if err := n.write(w); err != nil {
			return fmt.Errorf("hnsw export: node %d: %w", i, err)
		}
	}
	return nil
}

// Import reads a graph previously written by Export.
func (g *Graph[K]) Import(r io.Reader) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var magic uint32
	if err := binary.Read(r, binary.LittleEndian, &magic); err != nil {
		return fmt.Errorf("hnsw import: magic: %w", err)
	}
	if magic != hnswMagic {
		return fmt.Errorf("hnsw import: invalid magic %08x", magic)
	}

	var version uint8
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return fmt.Errorf("hnsw import: version: %w", err)
	}
	if version != hnswVersion {
		return fmt.Errorf("hnsw import: unsupported version %d", version)
	}

	var m, efC, entry, maxL int32
	if err := binary.Read(r, binary.LittleEndian, &m); err != nil {
		return fmt.Errorf("hnsw import: m: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &efC); err != nil {
		return fmt.Errorf("hnsw import: efC: %w", err)
	}
	var ml float64
	if err := binary.Read(r, binary.LittleEndian, &ml); err != nil {
		return fmt.Errorf("hnsw import: ml: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &entry); err != nil {
		return fmt.Errorf("hnsw import: entry: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &maxL); err != nil {
		return fmt.Errorf("hnsw import: maxL: %w", err)
	}

	g.m = int(m)
	g.efC = int(efC)
	g.ml = ml
	g.entry = int(entry)
	g.maxL = int(maxL)

	var nNodes int32
	if err := binary.Read(r, binary.LittleEndian, &nNodes); err != nil {
		return fmt.Errorf("hnsw import: node count: %w", err)
	}

	g.nodes = make([]hnswNode[K], nNodes)
	for i := range g.nodes {
		if err := g.nodes[i].read(r); err != nil {
			return fmt.Errorf("hnsw import: node %d: %w", i, err)
		}
	}
	return nil
}

// ---- internal types & helpers -----------------------------------------------

// hnswNode stores a single graph node.
// Fields are ordered for optimal memory alignment.
type hnswNode[K cmp.Ordered] struct {
	key   K
	conns [][]int // conns[layer] = list of neighbour indices
	vec   []float32
	level int
}

// write serialises a single node.
func (n *hnswNode[K]) write(w io.Writer) error {
	if err := writeKey(w, n.key); err != nil {
		return err
	}

	// vec
	if err := binary.Write(w, binary.LittleEndian, int32(len(n.vec))); err != nil { //nolint:gosec // bounded by vector dimension
		return fmt.Errorf("hnsw node write: vec len: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, n.vec); err != nil {
		return fmt.Errorf("hnsw node write: vec data: %w", err)
	}

	// level
	if err := binary.Write(w, binary.LittleEndian, int32(n.level)); err != nil { //nolint:gosec // level capped at 32
		return fmt.Errorf("hnsw node write: level: %w", err)
	}

	// conns
	if err := binary.Write(w, binary.LittleEndian, int32(len(n.conns))); err != nil { //nolint:gosec // bounded by level
		return fmt.Errorf("hnsw node write: conn layers: %w", err)
	}
	for li, layer := range n.conns {
		if err := binary.Write(w, binary.LittleEndian, int32(len(layer))); err != nil { //nolint:gosec // bounded by m
			return fmt.Errorf("hnsw node write: layer %d len: %w", li, err)
		}
		for _, c := range layer {
			if err := binary.Write(w, binary.LittleEndian, int32(c)); err != nil {
				return fmt.Errorf("hnsw node write: conn: %w", err)
			}
		}
	}
	return nil
}

// read deserialises a single node.
func (n *hnswNode[K]) read(r io.Reader) error {
	key, err := readKey[K](r)
	if err != nil {
		return err
	}
	n.key = key

	var vecLen int32
	if err := binary.Read(r, binary.LittleEndian, &vecLen); err != nil {
		return fmt.Errorf("hnsw node read: vec len: %w", err)
	}
	n.vec = make([]float32, vecLen)
	if err := binary.Read(r, binary.LittleEndian, n.vec); err != nil {
		return fmt.Errorf("hnsw node read: vec data: %w", err)
	}

	var level int32
	if err := binary.Read(r, binary.LittleEndian, &level); err != nil {
		return fmt.Errorf("hnsw node read: level: %w", err)
	}
	n.level = int(level)

	var nLayers int32
	if err := binary.Read(r, binary.LittleEndian, &nLayers); err != nil {
		return fmt.Errorf("hnsw node read: conn layers: %w", err)
	}
	n.conns = make([][]int, nLayers)
	for l := range n.conns {
		var nConns int32
		if err := binary.Read(r, binary.LittleEndian, &nConns); err != nil {
			return fmt.Errorf("hnsw node read: layer %d len: %w", l, err)
		}
		n.conns[l] = make([]int, nConns)
		for i := range n.conns[l] {
			var c int32
			if err := binary.Read(r, binary.LittleEndian, &c); err != nil {
				return fmt.Errorf("hnsw node read: conn: %w", err)
			}
			n.conns[l][i] = int(c)
		}
	}
	return nil
}

// ---- key codec (type-switched for cmp.Ordered) ------------------------------

// writeKeyTag writes the type tag byte for a key.
func writeKeyTag(w io.Writer, tag uint8) error {
	if err := binary.Write(w, binary.LittleEndian, tag); err != nil {
		return fmt.Errorf("hnsw key write: tag: %w", err)
	}
	return nil
}

// writeKey writes a cmp.Ordered key as a tagged binary value.
func writeKey[K cmp.Ordered](w io.Writer, key K) error { //nolint:gocognit,gocyclo,funlen // exhaustive type switch required for cmp.Ordered
	switch v := any(key).(type) {
	case int:
		if err := writeKeyTag(w, 1); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, int64(v)); err != nil {
			return fmt.Errorf("hnsw key write: int: %w", err)
		}
		return nil
	case int8:
		if err := writeKeyTag(w, 2); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, int64(v)); err != nil {
			return fmt.Errorf("hnsw key write: int8: %w", err)
		}
		return nil
	case int16:
		if err := writeKeyTag(w, 3); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, int64(v)); err != nil {
			return fmt.Errorf("hnsw key write: int16: %w", err)
		}
		return nil
	case int32:
		if err := writeKeyTag(w, 4); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, int64(v)); err != nil {
			return fmt.Errorf("hnsw key write: int32: %w", err)
		}
		return nil
	case int64:
		if err := writeKeyTag(w, 5); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			return fmt.Errorf("hnsw key write: int64: %w", err)
		}
		return nil
	case uint:
		if err := writeKeyTag(w, 6); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint64(v)); err != nil {
			return fmt.Errorf("hnsw key write: uint: %w", err)
		}
		return nil
	case uint8:
		if err := writeKeyTag(w, 7); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint64(v)); err != nil {
			return fmt.Errorf("hnsw key write: uint8: %w", err)
		}
		return nil
	case uint16:
		if err := writeKeyTag(w, 8); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint64(v)); err != nil {
			return fmt.Errorf("hnsw key write: uint16: %w", err)
		}
		return nil
	case uint32:
		if err := writeKeyTag(w, 9); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint64(v)); err != nil {
			return fmt.Errorf("hnsw key write: uint32: %w", err)
		}
		return nil
	case uint64:
		if err := writeKeyTag(w, 10); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			return fmt.Errorf("hnsw key write: uint64: %w", err)
		}
		return nil
	case float32:
		if err := writeKeyTag(w, 11); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			return fmt.Errorf("hnsw key write: float32: %w", err)
		}
		return nil
	case float64:
		if err := writeKeyTag(w, 12); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			return fmt.Errorf("hnsw key write: float64: %w", err)
		}
		return nil
	case string:
		if err := writeKeyTag(w, 13); err != nil {
			return err
		}
		return writeKeyString(w, v)
	default:
		return fmt.Errorf("hnsw: unsupported key type %T", key)
	}
}

// writeKeyString writes a string key payload (length-prefixed bytes).
func writeKeyString(w io.Writer, s string) error {
	b := []byte(s)
	if err := binary.Write(w, binary.LittleEndian, int32(len(b))); err != nil { //nolint:gosec // string length bounded by practical limits
		return fmt.Errorf("hnsw key write: string len: %w", err)
	}
	if _, err := w.Write(b); err != nil {
		return fmt.Errorf("hnsw key write: string data: %w", err)
	}
	return nil
}

// readKeySignedInt reads an int64 and converts it to the target signed integer type K.
func readKeySignedInt[K cmp.Ordered](r io.Reader) (K, error) {
	var zero K
	var v int64
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return zero, fmt.Errorf("hnsw key read: signed int: %w", err)
	}
	result, ok := convertInt[K](v).(K)
	if !ok {
		return zero, fmt.Errorf("hnsw key read: signed int type assertion failed")
	}
	return result, nil
}

// readKeyUnsignedInt reads a uint64 and converts it to the target unsigned integer type K.
func readKeyUnsignedInt[K cmp.Ordered](r io.Reader) (K, error) {
	var zero K
	var v uint64
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return zero, fmt.Errorf("hnsw key read: unsigned int: %w", err)
	}
	result, ok := convertUint[K](v).(K)
	if !ok {
		return zero, fmt.Errorf("hnsw key read: unsigned int type assertion failed")
	}
	return result, nil
}

// readKeyFloat32 reads a float32 key value.
func readKeyFloat32[K cmp.Ordered](r io.Reader) (K, error) {
	var zero K
	var v float32
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return zero, fmt.Errorf("hnsw key read: float32: %w", err)
	}
	result, ok := any(v).(K)
	if !ok {
		return zero, fmt.Errorf("hnsw key read: float32 type assertion failed")
	}
	return result, nil
}

// readKeyFloat64 reads a float64 key value.
func readKeyFloat64[K cmp.Ordered](r io.Reader) (K, error) {
	var zero K
	var v float64
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return zero, fmt.Errorf("hnsw key read: float64: %w", err)
	}
	result, ok := any(v).(K)
	if !ok {
		return zero, fmt.Errorf("hnsw key read: float64 type assertion failed")
	}
	return result, nil
}

// readKeyString reads a length-prefixed string key.
func readKeyString[K cmp.Ordered](r io.Reader) (K, error) {
	var zero K
	var length int32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return zero, fmt.Errorf("hnsw key read: string len: %w", err)
	}
	b := make([]byte, length)
	if _, err := io.ReadFull(r, b); err != nil {
		return zero, fmt.Errorf("hnsw key read: string data: %w", err)
	}
	result, ok := any(string(b)).(K)
	if !ok {
		return zero, fmt.Errorf("hnsw key read: string type assertion failed")
	}
	return result, nil
}

// readKey reads a key previously written by writeKey.
func readKey[K cmp.Ordered](r io.Reader) (K, error) {
	var zero K
	var tag uint8
	if err := binary.Read(r, binary.LittleEndian, &tag); err != nil {
		return zero, fmt.Errorf("hnsw key read: tag: %w", err)
	}
	switch tag {
	case 1, 2, 3, 4, 5: // signed int family
		return readKeySignedInt[K](r)
	case 6, 7, 8, 9, 10: // unsigned int family
		return readKeyUnsignedInt[K](r)
	case 11: // float32
		return readKeyFloat32[K](r)
	case 12: // float64
		return readKeyFloat64[K](r)
	case 13: // string
		return readKeyString[K](r)
	default:
		return zero, fmt.Errorf("hnsw: unknown key tag %d", tag)
	}
}

// convertInt converts int64 to the target cmp.Ordered integer type.
func convertInt[K cmp.Ordered](v int64) any {
	var zero K
	switch any(zero).(type) {
	case int:
		return int(v)
	case int8:
		return int8(v) //nolint:gosec // narrowing is intentional — caller controls serialised data
	case int16:
		return int16(v) //nolint:gosec // narrowing is intentional
	case int32:
		return int32(v) //nolint:gosec // narrowing is intentional
	case int64:
		return v
	default:
		return v
	}
}

// convertUint converts uint64 to the target cmp.Ordered unsigned type.
func convertUint[K cmp.Ordered](v uint64) any {
	var zero K
	switch any(zero).(type) {
	case uint:
		return uint(v)
	case uint8:
		return uint8(v) //nolint:gosec // narrowing is intentional
	case uint16:
		return uint16(v) //nolint:gosec // narrowing is intentional
	case uint32:
		return uint32(v) //nolint:gosec // narrowing is intentional
	case uint64:
		return v
	default:
		return v
	}
}

// ---- HNSW algorithm helpers -------------------------------------------------

// candidateItem is an item in the priority queue used during search.
type candidateItem struct {
	idx  int
	dist float32
}

// candidateHeap is a max-heap (farthest first) for use in beam search.
type candidateHeap []candidateItem

func (h candidateHeap) Len() int            { return len(h) }
func (h candidateHeap) Less(i, j int) bool  { return h[i].dist > h[j].dist } // max-heap
func (h candidateHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *candidateHeap) Push(x interface{}) { *h = append(*h, x.(candidateItem)) } //nolint:errcheck // heap.Interface: type is always candidateItem
func (h *candidateHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// minHeap is a min-heap (closest first) for candidate expansion.
type minHeap []candidateItem

func (h minHeap) Len() int            { return len(h) }
func (h minHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist } // min-heap
func (h minHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x interface{}) { *h = append(*h, x.(candidateItem)) } //nolint:errcheck // heap.Interface: type is always candidateItem
func (h *minHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// cosineDistance returns 1 - cosine_similarity (in [0,2]).
func cosineDistance(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 2.0
	}
	var dot, na, nb float32
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 2.0
	}
	sim := dot / (float32(math.Sqrt(float64(na))) * float32(math.Sqrt(float64(nb))))
	return 1.0 - sim
}

// randomLevel returns a random insertion level following the HNSW probability distribution.
func (g *Graph[K]) randomLevel() int {
	l := 0
	//nolint:gosec // math/rand is intentional: level sampling does not require cryptographic randomness
	for rand.Float64() < 0.5 && l < 32 {
		l++
	}
	_ = g.ml // ml is retained for future tuning; level cap is sufficient
	return l
}

// greedyClosest performs greedy descent on a single layer, returning the index
// of the closest node to q at that layer.
func (g *Graph[K]) greedyClosest(ep int, q []float32, layer int) int {
	best := ep
	bestDist := cosineDistance(g.nodes[ep].vec, q)
	for {
		moved := false
		if layer >= len(g.nodes[best].conns) {
			break
		}
		for _, nb := range g.nodes[best].conns[layer] {
			d := cosineDistance(g.nodes[nb].vec, q)
			if d < bestDist {
				bestDist = d
				best = nb
				moved = true
			}
		}
		if !moved {
			break
		}
	}
	return best
}

// searchLayer performs beam search on a given layer and returns candidates
// sorted closest-first (length ≤ ef).
func (g *Graph[K]) searchLayer(ep int, q []float32, ef int, layer int) []candidateItem { //nolint:gocognit // HNSW beam-search algorithm — complexity is inherent
	visited := make(map[int]struct{}, ef*2)
	visited[ep] = struct{}{}

	epDist := cosineDistance(g.nodes[ep].vec, q)

	// candidates: min-heap (closest first)
	cands := &minHeap{candidateItem{ep, epDist}}
	heap.Init(cands)

	// results: max-heap capped at ef (farthest at top for easy pruning)
	results := &candidateHeap{candidateItem{ep, epDist}}
	heap.Init(results)

	for cands.Len() > 0 {
		c := heap.Pop(cands).(candidateItem) //nolint:errcheck // heap always contains candidateItem
		// If closest candidate is farther than worst result, stop
		if results.Len() >= ef && c.dist > (*results)[0].dist {
			break
		}

		if layer >= len(g.nodes[c.idx].conns) {
			continue
		}

		for _, nb := range g.nodes[c.idx].conns[layer] {
			if _, seen := visited[nb]; seen {
				continue
			}
			visited[nb] = struct{}{}

			d := cosineDistance(g.nodes[nb].vec, q)
			if results.Len() < ef || d < (*results)[0].dist {
				heap.Push(cands, candidateItem{nb, d})
				heap.Push(results, candidateItem{nb, d})
				if results.Len() > ef {
					heap.Pop(results)
				}
			}
		}
	}

	// Convert results heap to sorted slice (closest first)
	out := make([]candidateItem, results.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(results).(candidateItem) //nolint:errcheck // heap always contains candidateItem
	}
	return out
}

// selectNeighbours picks the best m neighbours from a sorted candidate list.
func (g *Graph[K]) selectNeighbours(candidates []candidateItem, m int) []int {
	result := make([]int, 0, m)
	for _, c := range candidates {
		if len(result) >= m {
			break
		}
		result = append(result, c.idx)
	}
	return result
}

// distHeapFromList builds a sorted candidate list for a given node's connections.
func (g *Graph[K]) distHeapFromList(nodeIdx int, conns []int, _ int) []candidateItem {
	q := g.nodes[nodeIdx].vec
	result := make([]candidateItem, 0, len(conns))
	for _, nb := range conns {
		d := cosineDistance(g.nodes[nb].vec, q)
		result = append(result, candidateItem{nb, d})
	}
	// Sort closest-first using a min-heap pass
	h := minHeap(result)
	heap.Init(&h)
	sorted := make([]candidateItem, len(h))
	for i := range sorted {
		sorted[i] = heap.Pop(&h).(candidateItem) //nolint:errcheck // heap always contains candidateItem
	}
	return sorted
}
