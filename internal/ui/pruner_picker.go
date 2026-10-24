package ui

import (
	"github.com/atakang7/axon/v2"
	"github.com/charmbracelet/bubbles/list"
)

func modelPickerItems(s axon.Settings, freeOnly bool) []list.Item {
	var items []list.Item
	for name, p := range s.Providers {
		for _, model := range p.ModelNames() {
			if freeOnly {
				if _, err := s.NewFreePrunerClient(name, model); err != nil {
					continue
				}
			}
			items = append(items, modelItem{provider: name, model: model})
		}
	}
	return items
}
