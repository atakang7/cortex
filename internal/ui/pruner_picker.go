package ui

import (
	"github.com/atakang7/axon/v2"
	"github.com/charmbracelet/bubbles/list"
)

// modelPickerItems lists every configured model. A non-empty noneLabel adds a
// first row with no model, for roles that can be left to the default.
func modelPickerItems(s axon.Settings, noneLabel string) []list.Item {
	var items []list.Item
	if noneLabel != "" {
		items = append(items, modelItem{label: noneLabel})
	}
	for name, p := range s.Providers {
		for _, model := range p.ModelNames() {
			items = append(items, modelItem{provider: name, model: model})
		}
	}
	return items
}

func (m Model) picking() bool {
	return m.pickingModel || m.pickingPruner || m.pickingExplorer || m.pickingSession
}

func (m *Model) stopPicking() {
	m.pickingModel, m.pickingPruner, m.pickingExplorer, m.pickingSession = false, false, false, false
}

// selectPruner applies a /pruner choice and returns the notice for it.
func (m *Model) selectPruner(i modelItem) string {
	if i.model == "" {
		m.agent.SetPrunerModel(nil)
		m.prunerName = ""
	} else {
		client, err := m.settings.NewClient(i.provider, i.model)
		if err != nil {
			return "checkpoint model not changed: " + err.Error()
		}
		m.agent.SetPrunerModel(client)
		m.prunerName = i.model
	}
	if m.onPrunerChanged != nil {
		m.onPrunerChanged(i.provider, i.model)
	}
	return "checkpoints decided by " + orDefault(m.prunerName, "the main model")
}

// selectExplorer applies an /explorer choice and returns the notice for it.
func (m *Model) selectExplorer(i modelItem) string {
	if i.model == "" {
		m.agent.SetExplorerModel(nil)
		m.explorerName = ""
	} else {
		client, err := m.settings.NewClient(i.provider, i.model)
		if err != nil {
			return "explorer not changed: " + err.Error()
		}
		m.agent.SetExplorerModel(client)
		m.explorerName = i.model
	}
	if m.onExplorerChanged != nil {
		m.onExplorerChanged(i.provider, i.model)
	}
	if m.explorerName == "" {
		return "explorer off"
	}
	return "explorer: " + m.explorerName + " (read-only)"
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
