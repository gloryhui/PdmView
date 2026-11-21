package appui

import (
	"fmt"
	"strconv"
	"strings"

	"pdmview/internal/app"
	"pdmview/internal/i18n"
	"pdmview/internal/pdm"

	"github.com/egoist/mygo/ui"
)

func renderTableDetail(
	c *ui.Context,
	state *app.State,
	t *ui.Theme,
	lang string,
	detail TableDetail,
	unplaced bool,
) {
	name := firstNonEmpty(detail.Name, detail.Code, detail.ID)
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, name).Key(DetailTableKey).FillWidth().Wrap().
			FontSize(14).Bold().TextColor(t.Text).Lang(lang)
		if unplaced {
			ui.Text(c, state.Text(i18n.KeyNotOnDiagram, nil)).
				FillWidth().Wrap().FontSize(11).TextColor(t.TextMuted).Lang(lang)
		}
		detailProperty(c, DetailCodeKey, state.Text(i18n.KeyDetailCode, nil), valueOrDash(detail.Code), t, lang)
		packagePath := packagePathText(detail.PackagePath)
		if packagePath == "" {
			packagePath = state.Text(i18n.KeyModelRoot, nil)
		}
		detailProperty(c, DetailPackageKey, state.Text(i18n.KeyDetailPackage, nil), packagePath, t, lang)
		detailProperty(c, DetailCommentKey, state.Text(i18n.KeyDetailComment, nil), optionalValue(detail.Comment), t, lang)

		detailSection(c, DetailColumnsKey, state.Text(i18n.KeyDetailColumns, nil), t, lang, func() {
			for offset, column := range detail.Columns {
				renderColumnDetail(c, state, t, lang, column, offset)
			}
		})
		detailSection(c, DetailKeysKey, state.Text(i18n.KeyDetailKeys, nil), t, lang, func() {
			for offset, key := range detail.Keys {
				id := detailID(key.ID, offset)
				ui.Column(c).FillWidth().Gap(2).Children(func() {
					mode := ""
					if key.Primary {
						mode = " · " + state.Text(i18n.KeyDetailPrimary, nil)
					}
					ui.Text(c, firstNonEmpty(key.Name, key.Code, key.ID, "—")+mode).Key(DetailKeyKey(id)).FillWidth().Wrap().
						FontSize(11).Bold().TextColor(t.Text).Lang(lang)
					detailProperty(c, DetailKeyKey(id)+".columns",
						state.Text(i18n.KeyDetailColumns, nil), refListText(state, key.Columns), t, lang)
					if key.Comment.Present {
						detailProperty(c, DetailKeyKey(id)+".comment",
							state.Text(i18n.KeyDetailComment, nil), optionalValue(key.Comment), t, lang)
					}
				})
			}
		})
		detailSection(c, DetailIndexesKey, state.Text(i18n.KeyDetailIndexes, nil), t, lang, func() {
			for offset, index := range detail.Indexes {
				id := detailID(index.ID, offset)
				ui.Column(c).FillWidth().Gap(2).Children(func() {
					ui.Text(c, firstNonEmpty(index.Name, index.Code, index.ID, "—")).Key(DetailIndexKey(id)).FillWidth().Wrap().
						FontSize(11).Bold().TextColor(t.Text).Lang(lang)
					detailProperty(c, DetailIndexKey(id)+".unique",
						state.Text(i18n.KeyDetailUnique, nil), boolValue(state, index.Unique), t, lang)
					for columnOffset, column := range index.Columns {
						value := detailRefText(state, column.Column)
						sortOrder := optionalValue(column.SortOrder)
						if column.SortOrder.Present {
							value += " · " + state.Text(i18n.KeyDetailSort, nil) + ": " + sortOrder
						}
						detailProperty(c, DetailIndexColumnKey(id, columnOffset),
							fmt.Sprintf("%d", columnOffset+1), value, t, lang)
					}
				})
			}
		})
		detailSection(c, DetailForeignKeysKey, state.Text(i18n.KeyDetailForeignKeys, nil), t, lang, func() {
			for offset, foreignKey := range detail.ForeignKeys {
				id := detailID(foreignKey.ID, offset)
				ui.Column(c).FillWidth().Gap(2).Children(func() {
					ui.Text(c, firstNonEmpty(foreignKey.Name, foreignKey.Code, foreignKey.ID, "—")).Key(DetailForeignKeyKey(id)).FillWidth().Wrap().
						FontSize(11).Bold().TextColor(t.Text).Lang(lang)
					detailProperty(c, DetailForeignKeyKey(id)+".parent",
						state.Text(i18n.KeyDetailParent, nil), detailRefText(state, foreignKey.ParentTable), t, lang)
					detailProperty(c, DetailForeignKeyKey(id)+".child",
						state.Text(i18n.KeyDetailChild, nil), detailRefText(state, foreignKey.ChildTable), t, lang)
					detailProperty(c, DetailForeignKeyKey(id)+".key",
						state.Text(i18n.KeyDetailKeys, nil), detailRefText(state, foreignKey.ParentKey), t, lang)
					for joinOffset, join := range foreignKey.Joins {
						value := detailRefText(state, join.ChildColumn) + " → " + detailRefText(state, join.ParentColumn)
						detailProperty(c, DetailJoinKey(id, joinOffset), fmt.Sprintf("%d", joinOffset+1), value, t, lang)
					}
					for _, property := range []struct {
						key   string
						label i18n.Key
						value pdm.OptionalText
					}{
						{key: DetailForeignKeyKey(id) + ".update", label: i18n.KeyDetailOnUpdate, value: foreignKey.UpdateConstraint},
						{key: DetailForeignKeyKey(id) + ".delete", label: i18n.KeyDetailOnDelete, value: foreignKey.DeleteConstraint},
						{key: DetailForeignKeyKey(id) + ".cardinality", label: i18n.KeyDetailCardinality, value: foreignKey.Cardinality},
					} {
						if property.value.Present {
							detailProperty(c, property.key, state.Text(property.label, nil), optionalValue(property.value), t, lang)
						}
					}
				})
			}
		})
	})
}

func renderColumnDetail(
	c *ui.Context,
	state *app.State,
	t *ui.Theme,
	lang string,
	column ColumnDetail,
	offset int,
) {
	id := detailID(column.ID, offset)
	ui.Column(c).FillWidth().Gap(2).Children(func() {
		markers := ""
		if column.PrimaryKey {
			markers += " · " + state.Text(i18n.KeyDetailPK, nil)
		}
		if column.ForeignKey {
			markers += " · " + state.Text(i18n.KeyDetailFK, nil)
		}
		ui.Text(c, strconv.Itoa(offset+1)+". "+valueOrDash(column.Name)+markers).Key(DetailColumnKey(id)).FillWidth().Wrap().
			FontSize(11).Bold().TextColor(t.Text).Lang(lang)
		detailProperty(c, DetailColumnPropertyKey(id, "code"),
			state.Text(i18n.KeyDetailCode, nil), valueOrDash(column.Code), t, lang)
		detailProperty(c, DetailColumnPropertyKey(id, "type"),
			state.Text(i18n.KeyDetailType, nil), optionalValue(column.DataType), t, lang)
		detailProperty(c, DetailColumnPropertyKey(id, "length"),
			state.Text(i18n.KeyDetailLength, nil), optionalValue(column.Length), t, lang)
		detailProperty(c, DetailColumnPropertyKey(id, "precision"),
			state.Text(i18n.KeyDetailPrecision, nil), optionalValue(column.Precision), t, lang)
		detailProperty(c, DetailColumnPropertyKey(id, "scale"),
			state.Text(i18n.KeyDetailScale, nil), optionalValue(column.Scale), t, lang)
		detailProperty(c, DetailColumnPropertyKey(id, "null"),
			state.Text(i18n.KeyDetailNull, nil), boolValue(state, column.Nullable), t, lang)
		detailProperty(c, DetailColumnPropertyKey(id, "default"),
			state.Text(i18n.KeyDetailDefault, nil), optionalValue(column.DefaultValue), t, lang)
		detailProperty(c, DetailColumnPropertyKey(id, "identity"),
			state.Text(i18n.KeyDetailIdentity, nil), boolValue(state, column.Identity), t, lang)
		if column.Comment.Present {
			detailProperty(c, DetailColumnPropertyKey(id, "comment"),
				state.Text(i18n.KeyDetailComment, nil), optionalValue(column.Comment), t, lang)
		}
	})
}

func detailSection(
	c *ui.Context,
	key, label string,
	t *ui.Theme,
	lang string,
	children func(),
) {
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		ui.Text(c, label).Key(key).FillWidth().FontSize(12).Bold().
			TextColor(t.Text).Lang(lang).Role(ui.RoleHeading).Level(3)
		children()
	})
}

func detailProperty(
	c *ui.Context,
	key, label, value string,
	t *ui.Theme,
	lang string,
) {
	ui.Row(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, label).Width(66).Shrink(0).Wrap().FontSize(11).
			TextColor(t.TextMuted).Lang(lang)
		ui.Text(c, value).Key(key).FillWidth().Wrap().FontSize(11).
			TextColor(t.Text).Lang(lang)
	})
}

func optionalValue(value pdm.OptionalText) string {
	if !value.Present {
		return "—"
	}
	if value.Value == "" {
		return `""`
	}
	return value.Value
}

func boolValue(state *app.State, value *bool) string {
	if value == nil {
		return "—"
	}
	if *value {
		return state.Text(i18n.KeyDetailYes, nil)
	}
	return state.Text(i18n.KeyDetailNo, nil)
}

func valueOrDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func detailRefText(state *app.State, ref DetailRef) string {
	if ref.ID == "" {
		return "—"
	}
	if ref.Resolved {
		return firstNonEmpty(ref.Name, ref.Code, ref.ID)
	}
	return ref.ID + " · " + state.Text(i18n.KeyDetailUnresolved, nil)
}

func refListText(state *app.State, refs []DetailRef) string {
	if len(refs) == 0 {
		return "—"
	}
	names := make([]string, len(refs))
	for offset, ref := range refs {
		names[offset] = detailRefText(state, ref)
	}
	return strings.Join(names, ", ")
}

func detailID(id string, offset int) string {
	if id != "" {
		return id
	}
	return "index-" + strconv.Itoa(offset)
}
