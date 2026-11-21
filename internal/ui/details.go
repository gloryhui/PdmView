package appui

import (
	"strconv"
	"strings"

	"pdmview/internal/pdm"
)

const (
	DetailTableKey       = SelectedNodeKey
	DetailCodeKey        = "pdmview.details.code"
	DetailCommentKey     = "pdmview.details.comment"
	DetailPackageKey     = "pdmview.details.package"
	DetailColumnsKey     = "pdmview.details.columns"
	DetailKeysKey        = "pdmview.details.keys"
	DetailIndexesKey     = "pdmview.details.indexes"
	DetailForeignKeysKey = "pdmview.details.foreign-keys"
)

func DetailColumnKey(id string) string {
	return "pdmview.details.column." + id
}

func DetailColumnPropertyKey(id, property string) string {
	return DetailColumnKey(id) + "." + property
}

func DetailKeyKey(id string) string {
	return "pdmview.details.key." + id
}

func DetailIndexKey(id string) string {
	return "pdmview.details.index." + id
}

func DetailIndexColumnKey(indexID string, ordinal int) string {
	return DetailIndexKey(indexID) + ".column." + strconv.Itoa(ordinal)
}

func DetailForeignKeyKey(id string) string {
	return "pdmview.details.foreign-key." + id
}

func DetailJoinKey(id string, ordinal int) string {
	return DetailForeignKeyKey(id) + ".join." + strconv.Itoa(ordinal)
}

type DetailRef struct {
	ID       string
	Name     string
	Code     string
	Resolved bool
}

type ColumnDetail struct {
	pdm.Column
	Nullable   *bool
	PrimaryKey bool
	ForeignKey bool
}

type KeyDetail struct {
	pdm.Metadata
	Primary bool
	Columns []DetailRef
}

type IndexColumnDetail struct {
	pdm.Metadata
	Column    DetailRef
	SortOrder pdm.OptionalText
}

type IndexDetail struct {
	pdm.Metadata
	Unique  *bool
	Columns []IndexColumnDetail
}

type JoinDetail struct {
	pdm.Metadata
	ParentColumn DetailRef
	ChildColumn  DetailRef
}

type ForeignKeyDetail struct {
	pdm.Metadata
	ParentTable      DetailRef
	ChildTable       DetailRef
	ParentKey        DetailRef
	Joins            []JoinDetail
	UpdateConstraint pdm.OptionalText
	DeleteConstraint pdm.OptionalText
	Cardinality      pdm.OptionalText
}

type TableDetail struct {
	pdm.Metadata
	PrimaryKeyID string
	PackagePath  []DetailRef
	Columns      []ColumnDetail
	Keys         []KeyDetail
	Indexes      []IndexDetail
	ForeignKeys  []ForeignKeyDetail
}

type detailObjectOwner struct {
	ref     DetailRef
	tableID string
}

type tableDetailIndex struct {
	tables            map[string]*pdm.Table
	columns           map[string]detailObjectOwner
	keys              map[string]detailObjectOwner
	packagePaths      map[string][]DetailRef
	referencesByChild map[string][]pdm.Reference
}

func buildTableDetailIndex(model *pdm.Model) *tableDetailIndex {
	index := &tableDetailIndex{
		tables:            make(map[string]*pdm.Table),
		columns:           make(map[string]detailObjectOwner),
		keys:              make(map[string]detailObjectOwner),
		packagePaths:      make(map[string][]DetailRef),
		referencesByChild: make(map[string][]pdm.Reference),
	}
	if model != nil {
		collectDetailModel(model.Contents, nil, index)
	}
	return index
}

func (index *tableDetailIndex) resolve(tableID string) (TableDetail, bool) {
	if index == nil || tableID == "" {
		return TableDetail{}, false
	}

	table, ok := index.tables[tableID]
	if !ok {
		return TableDetail{}, false
	}
	detail := TableDetail{
		Metadata:     table.Metadata,
		PrimaryKeyID: table.PrimaryKeyID,
		PackagePath:  index.packagePaths[tableID],
	}

	primaryColumns := make(map[string]bool)
	for _, key := range table.Keys {
		if key.ID == "" || key.ID != table.PrimaryKeyID {
			continue
		}
		for _, columnID := range key.ColumnIDs {
			if index.columns[columnID].tableID == table.ID {
				primaryColumns[columnID] = true
			}
		}
	}
	foreignColumns := make(map[string]bool)
	for _, reference := range index.referencesByChild[table.ID] {
		for _, join := range reference.Joins {
			if index.columns[join.ChildColumnID].tableID == table.ID {
				foreignColumns[join.ChildColumnID] = true
			}
		}
	}

	for _, column := range table.Columns {
		var nullable *bool
		if column.Mandatory != nil {
			value := !*column.Mandatory
			nullable = &value
		}
		detail.Columns = append(detail.Columns, ColumnDetail{
			Column:     column,
			Nullable:   nullable,
			PrimaryKey: primaryColumns[column.ID],
			ForeignKey: foreignColumns[column.ID],
		})
	}
	for _, key := range table.Keys {
		keyDetail := KeyDetail{
			Metadata: key.Metadata,
			Primary:  key.ID != "" && key.ID == table.PrimaryKeyID,
		}
		for _, columnID := range key.ColumnIDs {
			keyDetail.Columns = append(keyDetail.Columns, resolveOwnedDetailRef(index.columns, columnID, table.ID))
		}
		detail.Keys = append(detail.Keys, keyDetail)
	}
	for _, sourceIndex := range table.Indexes {
		indexDetail := IndexDetail{
			Metadata: sourceIndex.Metadata,
			Unique:   sourceIndex.Unique,
		}
		for _, column := range sourceIndex.Columns {
			indexDetail.Columns = append(indexDetail.Columns, IndexColumnDetail{
				Metadata:  column.Metadata,
				Column:    resolveOwnedDetailRef(index.columns, column.ColumnID, table.ID),
				SortOrder: column.SortOrder,
			})
		}
		detail.Indexes = append(detail.Indexes, indexDetail)
	}
	for _, reference := range index.referencesByChild[table.ID] {
		foreignKey := ForeignKeyDetail{
			Metadata:         reference.Metadata,
			ParentTable:      index.resolveTableRef(reference.ParentTableID),
			ChildTable:       index.resolveTableRef(reference.ChildTableID),
			ParentKey:        resolveOwnedDetailRef(index.keys, reference.ParentKeyID, reference.ParentTableID),
			UpdateConstraint: reference.UpdateConstraint,
			DeleteConstraint: reference.DeleteConstraint,
			Cardinality:      reference.Cardinality,
		}
		for _, join := range reference.Joins {
			foreignKey.Joins = append(foreignKey.Joins, JoinDetail{
				Metadata:     join.Metadata,
				ParentColumn: resolveOwnedDetailRef(index.columns, join.ParentColumnID, reference.ParentTableID),
				ChildColumn:  resolveOwnedDetailRef(index.columns, join.ChildColumnID, reference.ChildTableID),
			})
		}
		detail.ForeignKeys = append(detail.ForeignKeys, foreignKey)
	}
	return detail, true
}

func collectDetailModel(
	contents pdm.Contents,
	path []DetailRef,
	index *tableDetailIndex,
) {
	for offset := range contents.Tables {
		table := &contents.Tables[offset]
		if table.ID == "" || index.tables[table.ID] != nil {
			continue
		}
		index.tables[table.ID] = table
		index.packagePaths[table.ID] = path
		for _, column := range table.Columns {
			if column.ID != "" {
				if _, exists := index.columns[column.ID]; !exists {
					index.columns[column.ID] = detailObjectOwner{tableID: table.ID, ref: resolvedDetailRef(column.Metadata)}
				}
			}
		}
		for _, key := range table.Keys {
			if key.ID != "" {
				if _, exists := index.keys[key.ID]; !exists {
					index.keys[key.ID] = detailObjectOwner{
						ref:     resolvedDetailRef(key.Metadata),
						tableID: table.ID,
					}
				}
			}
		}
	}
	for _, reference := range contents.References {
		if reference.ChildTableID != "" {
			index.referencesByChild[reference.ChildTableID] = append(index.referencesByChild[reference.ChildTableID], reference)
		}
	}
	for _, pkg := range contents.Packages {
		next := append(append([]DetailRef(nil), path...), resolvedDetailRef(pkg.Metadata))
		collectDetailModel(pkg.Contents, next, index)
	}
}

func resolveOwnedDetailRef(objects map[string]detailObjectOwner, id, expectedTableID string) DetailRef {
	if owner, ok := objects[id]; ok && owner.tableID == expectedTableID {
		return owner.ref
	}
	return DetailRef{ID: id}
}

func (index *tableDetailIndex) resolveTableRef(id string) DetailRef {
	if table, ok := index.tables[id]; ok {
		return resolvedDetailRef(table.Metadata)
	}
	return DetailRef{ID: id}
}

func resolvedDetailRef(metadata pdm.Metadata) DetailRef {
	return DetailRef{ID: metadata.ID, Name: metadata.Name, Code: metadata.Code, Resolved: true}
}

func packagePathText(path []DetailRef) string {
	names := make([]string, 0, len(path))
	for _, ref := range path {
		names = append(names, firstNonEmpty(ref.Name, ref.Code, ref.ID))
	}
	return strings.Join(names, " / ")
}
