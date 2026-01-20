package navigation

// TabController coordinates tab selection state.
type TabController struct {
	index     int
	listeners map[int]func(int)
	nextID    int
}

// NewTabController creates a controller with the initial index.
func NewTabController(initialIndex int) *TabController {
	return &TabController{index: initialIndex}
}

// Index returns the current tab index.
func (c *TabController) Index() int {
	if c == nil {
		return 0
	}
	return c.index
}

// SetIndex updates the active tab index.
func (c *TabController) SetIndex(index int) {
	if c == nil || c.index == index {
		return
	}
	c.index = index
	for _, listener := range c.listeners {
		listener(index)
	}
}

// AddListener registers a listener.
// Returns an unsubscribe function.
func (c *TabController) AddListener(listener func(int)) func() {
	if c.listeners == nil {
		c.listeners = make(map[int]func(int))
	}
	id := c.nextID
	c.nextID++
	c.listeners[id] = listener
	return func() {
		delete(c.listeners, id)
	}
}
