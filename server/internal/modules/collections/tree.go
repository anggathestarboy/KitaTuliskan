package collections

// = CollectionTreeService + Structure\TreeService (operasi node): hierarki entry per site untuk collection "structured".
// Tree disimpan di tabel trees (type=collection, handle=collection, locale=site); hanya id + children yang dipakai.

import (
	"errors"
	"math"

	"draftcms-lite/internal/modules/structure"
)

type node = structure.Item

func findNode(nodes []node, id string) *node {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
		if f := findNode(nodes[i].Children, id); f != nil {
			return f
		}
	}
	return nil
}

func depthOf(nodes []node, id string, depth int) int {
	for _, n := range nodes {
		if n.ID == id {
			return depth
		}
		if d := depthOf(n.Children, id, depth+1); d > 0 {
			return d
		}
	}
	return 0
}

func heightOf(n node) int {
	h := 0
	for _, c := range n.Children {
		h = max(h, heightOf(c))
	}
	return h + 1
}

func removeNode(nodes []node, id string) []node {
	out := []node{}
	for _, n := range nodes {
		if n.ID == id {
			continue
		}
		n.Children = removeNode(n.Children, id)
		out = append(out, n)
	}
	return out
}

// extract = extractNode: (tree tanpa node itu, node-nya atau nil).
func extract(nodes []node, id string) ([]node, *node) {
	var got *node
	out := []node{}
	for _, n := range nodes {
		if got == nil && n.ID == id {
			c := n
			got = &c
			continue
		}
		kids, g := extract(n.Children, id)
		n.Children = kids
		if g != nil {
			got = g
		}
		out = append(out, n)
	}
	return out, got
}

func insertAt(list []node, pos int, n node) []node {
	pos = max(0, min(pos, len(list)))
	return append(list[:pos], append([]node{n}, list[pos:]...)...)
}

// place memasang n sebagai anak parent (nil = root) di posisi pos.
func place(nodes []node, parent *string, pos int, n node) []node {
	if parent == nil {
		return insertAt(nodes, pos, n)
	}
	for i := range nodes {
		if nodes[i].ID == *parent {
			nodes[i].Children = insertAt(nodes[i].Children, pos, n)
			return nodes
		}
		nodes[i].Children = place(nodes[i].Children, parent, pos, n)
	}
	return nodes
}

func addNode(nodes []node, parent *string, n node, maxDepth *int) ([]node, error) {
	if parent == nil {
		return append(nodes, n), nil
	}
	d := depthOf(nodes, *parent, 1)
	if d == 0 {
		return nil, errors.New("Parent node not found: " + *parent)
	}
	if maxDepth != nil && d+1 > *maxDepth {
		return nil, errors.New("Maximum tree depth reached.")
	}
	return place(nodes, parent, math.MaxInt32, n), nil
}

func moveNode(nodes []node, id string, parent *string, pos int, maxDepth *int) ([]node, error) {
	rest, n := extract(nodes, id)
	if n == nil {
		return nil, errors.New("Node not found: " + id)
	}
	target := 1
	if parent != nil {
		d := depthOf(rest, *parent, 1)
		if d == 0 {
			return nil, errors.New("Parent node not found: " + *parent)
		}
		target = d + 1
	}
	if maxDepth != nil && target+heightOf(*n)-1 > *maxDepth {
		return nil, errors.New("Moving this item here would exceed the maximum tree depth.")
	}
	return place(rest, parent, pos, *n), nil
}

func (m *mod) loadTree(handle, site string) []node {
	return structure.LoadItems(m.a.DB, handle, "collection", site)
}

// parentMap = parentIdOf() untuk semua entry sekaligus: id → id induk ("" = root / belum tercatat).
func (m *mod) parentMap(handle, site string) map[string]string {
	out := map[string]string{}
	var walk func(nodes []node, parent string)
	walk = func(nodes []node, parent string) {
		for _, n := range nodes {
			out[n.ID] = parent
			walk(n.Children, n.ID)
		}
	}
	walk(m.loadTree(handle, site), "")
	return out
}

// setParent: induk baru (nil = root); entry/induk yang belum tercatat didaftarkan dulu. Melanggar max depth → error.
func (m *mod) setParent(handle, site, id string, parent *string, maxDepth *int) error {
	tree := m.loadTree(handle, site)
	var err error
	if parent != nil && findNode(tree, *parent) == nil {
		tree = append(tree, node{ID: *parent})
	}
	if findNode(tree, id) != nil {
		tree, err = moveNode(tree, id, parent, math.MaxInt32, maxDepth)
	} else {
		tree, err = addNode(tree, parent, node{ID: id}, maxDepth)
	}
	if err != nil {
		return err
	}
	return structure.SaveItems(m.a.DB, handle, "collection", site, tree)
}

func (m *mod) removeFromTree(handle, site, id string) {
	tree := m.loadTree(handle, site)
	if findNode(tree, id) != nil {
		structure.SaveItems(m.a.DB, handle, "collection", site, removeNode(tree, id))
	}
}
