package config

import (
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

// Priorities stores the ordered list of work item IDs for priority sorting.
// Items at the beginning of the Order slice have higher priority.
type Priorities struct {
	Order   []int `yaml:"order"`
	changed bool  // tracks if changes need to be saved
}

// LoadPriorities loads the priority list from ~/.config/adoBizInt/priorities.yaml
// Returns an empty Priorities struct if the file doesn't exist.
func LoadPriorities() (*Priorities, error) {
	p := &Priorities{
		Order: []int{},
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return p, nil // Return empty priorities on error
	}

	prioritiesPath := filepath.Join(home, ".config", "adoBizInt", "priorities.yaml")
	data, err := os.ReadFile(prioritiesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return p, nil // File doesn't exist yet, return empty
		}
		return p, err
	}

	if err := yaml.Unmarshal(data, p); err != nil {
		return p, err
	}

	return p, nil
}

// Save persists the priority list to ~/.config/adoBizInt/priorities.yaml
// Only writes if there have been changes since last load/save.
func (p *Priorities) Save() error {
	if !p.changed {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(home, ".config", "adoBizInt")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	prioritiesPath := filepath.Join(configDir, "priorities.yaml")

	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}

	if err := os.WriteFile(prioritiesPath, data, 0644); err != nil {
		return err
	}

	p.changed = false
	return nil
}

// AddToTop adds a work item ID to the top of the priority list.
// If the ID already exists, it is moved to the top.
func (p *Priorities) AddToTop(id int) {
	// Remove if already exists
	p.Remove(id)
	// Add to top
	p.Order = append([]int{id}, p.Order...)
	p.changed = true
}

// Remove removes a work item ID from the priority list.
func (p *Priorities) Remove(id int) {
	idx := p.indexOf(id)
	if idx == -1 {
		return
	}
	p.Order = slices.Delete(p.Order, idx, idx+1)
	p.changed = true
}

// MoveUp moves a work item ID up one position in the priority list.
// Returns true if the item was moved, false if it was already at the top or not found.
func (p *Priorities) MoveUp(id int) bool {
	idx := p.indexOf(id)
	if idx <= 0 {
		return false
	}
	// Swap with previous item
	p.Order[idx], p.Order[idx-1] = p.Order[idx-1], p.Order[idx]
	p.changed = true
	return true
}

// MoveDown moves a work item ID down one position in the priority list.
// Returns true if the item was moved, false if it was already at the bottom or not found.
func (p *Priorities) MoveDown(id int) bool {
	idx := p.indexOf(id)
	if idx == -1 || idx >= len(p.Order)-1 {
		return false
	}
	// Swap with next item
	p.Order[idx], p.Order[idx+1] = p.Order[idx+1], p.Order[idx]
	p.changed = true
	return true
}

// GetPosition returns the 0-based position of a work item ID in the priority list.
// Returns -1 if the ID is not in the list.
func (p *Priorities) GetPosition(id int) int {
	return p.indexOf(id)
}

// Contains returns true if the work item ID is in the priority list.
func (p *Priorities) Contains(id int) bool {
	return p.indexOf(id) != -1
}

// Clean removes any work item IDs from the priority list that are not in the validIDs set.
// This is used to remove IDs for work items that no longer exist.
func (p *Priorities) Clean(validIDs map[int]bool) {
	var cleaned []int
	for _, id := range p.Order {
		if validIDs[id] {
			cleaned = append(cleaned, id)
		}
	}
	if len(cleaned) != len(p.Order) {
		p.Order = cleaned
		p.changed = true
	}
}

// indexOf returns the index of id in Order, or -1 if not found.
func (p *Priorities) indexOf(id int) int {
	for i, v := range p.Order {
		if v == id {
			return i
		}
	}
	return -1
}
