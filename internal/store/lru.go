package store

// lruNode represents an entry in the LRU doubly-linked list.
type lruNode struct {
	key  string
	prev *lruNode
	next *lruNode
}

// lruPolicy implements the EvictionPolicy interface using an O(1) doubly-linked list + map.
// Head represents the Most Recently Used (MRU) key.
// Tail represents the Least Recently Used (LRU) key.
type lruPolicy struct {
	nodes map[string]*lruNode
	head  *lruNode
	tail  *lruNode
}

// NewLRUPolicy creates a new instance of the LRU eviction policy.
func NewLRUPolicy() EvictionPolicy {
	return &lruPolicy{
		nodes: make(map[string]*lruNode),
	}
}

// OnGet marks a key as most recently used.
func (p *lruPolicy) OnGet(key string) {
	if node, exists := p.nodes[key]; exists {
		p.moveToHead(node)
	}
}

// OnSet inserts a new key or promotes an existing key to MRU.
func (p *lruPolicy) OnSet(key string) {
	if node, exists := p.nodes[key]; exists {
		p.moveToHead(node)
		return
	}

	newNode := &lruNode{key: key}
	p.nodes[key] = newNode
	p.pushHead(newNode)
}

// OnDelete removes a key from the LRU tracker.
func (p *lruPolicy) OnDelete(key string) {
	if node, exists := p.nodes[key]; exists {
		p.removeNode(node)
		delete(p.nodes, key)
	}
}

// SelectEvict returns the candidate least recently used key at the tail of the list.
func (p *lruPolicy) SelectEvict() (string, bool) {
	if p.tail == nil {
		return "", false
	}
	return p.tail.key, true
}

// Clear resets the LRU state.
func (p *lruPolicy) Clear() {
	p.nodes = make(map[string]*lruNode)
	p.head = nil
	p.tail = nil
}

// pushHead inserts a new node at the head of the doubly linked list.
func (p *lruPolicy) pushHead(node *lruNode) {
	node.prev = nil
	node.next = p.head

	if p.head != nil {
		p.head.prev = node
	}
	p.head = node

	if p.tail == nil {
		p.tail = node
	}
}

// removeNode detaches a node from its current position in the doubly linked list.
func (p *lruPolicy) removeNode(node *lruNode) {
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		p.head = node.next // Removing head
	}

	if node.next != nil {
		node.next.prev = node.prev
	} else {
		p.tail = node.prev // Removing tail
	}

	node.prev = nil
	node.next = nil
}

// moveToHead promotes an existing node to the head of the list.
func (p *lruPolicy) moveToHead(node *lruNode) {
	if p.head == node {
		return // Already MRU
	}
	p.removeNode(node)
	p.pushHead(node)
}
