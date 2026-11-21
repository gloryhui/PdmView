package appui

import (
	"pdmview/internal/pdm"
)

const (
	ModelRootKey = "pdmview.workspace.model"
)

func PackageNavKey(id string) string {
	return "pdmview.workspace.package." + id
}

func TableNavKey(id string) string {
	return "pdmview.workspace.table." + id
}

type modelTreeNode struct {
	key      string
	label    string
	tableID  string
	children []string
}

type modelCatalog struct {
	root   string
	nodes  map[string]modelTreeNode
	parent map[string]string
}

func buildModelCatalog(model *pdm.Model, rootLabel string) *modelCatalog {
	catalog := &modelCatalog{
		root:   ModelRootKey,
		nodes:  make(map[string]modelTreeNode),
		parent: make(map[string]string),
	}
	root := modelTreeNode{key: ModelRootKey, label: rootLabel}
	if model != nil && model.ID != "" {
		root.label = firstNonEmpty(model.Name, model.Code, model.ID, rootLabel)
	}
	catalog.nodes[ModelRootKey] = root
	if model != nil {
		catalog.addContents(model.Contents, ModelRootKey, 0)
	}
	return catalog
}

func (c *modelCatalog) addContents(contents pdm.Contents, parent string, depth int) {
	node := c.nodes[parent]
	for pkgOffset := range contents.Packages {
		pkg := &contents.Packages[pkgOffset]
		id := pkg.ID
		if id == "" {
			continue
		}
		key := PackageNavKey(id)
		if _, exists := c.nodes[key]; exists {
			continue
		}
		label := firstNonEmpty(pkg.Name, pkg.Code, pkg.ID, id)
		c.nodes[key] = modelTreeNode{key: key, label: label}
		c.parent[key] = parent
		node.children = append(node.children, key)
		c.addContents(pkg.Contents, key, depth+1)
	}
	for _, table := range contents.Tables {
		if table.ID == "" {
			continue
		}
		key := TableNavKey(table.ID)
		if _, exists := c.nodes[key]; exists {
			continue
		}
		label := firstNonEmpty(table.Name, table.Code, table.ID)
		c.nodes[key] = modelTreeNode{key: key, label: label, tableID: table.ID}
		c.parent[key] = parent
		node.children = append(node.children, key)
	}
	c.nodes[parent] = node
}

func (c *modelCatalog) children(key string) []string {
	if c == nil {
		return nil
	}
	node, ok := c.nodes[key]
	if !ok || len(node.children) == 0 {
		return nil
	}
	return node.children
}

func (c *modelCatalog) tableName(id string) string {
	if c == nil {
		return id
	}
	if node, ok := c.nodes[TableNavKey(id)]; ok {
		return node.label
	}
	return id
}
