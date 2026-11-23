package sqlgen

import (
	"reflect"
	"strings"
	"testing"

	"pdmview/internal/pdm"
)

func TestGenerateTablePreviewModes(t *testing.T) {
	if TablePreviewFull != 0 ||
		TablePreviewStandard != 1 ||
		TablePreviewBare != 2 {
		t.Fatalf(
			"preview mode values = full %d, standard %d, bare %d",
			TablePreviewFull, TablePreviewStandard, TablePreviewBare,
		)
	}

	table := previewCommentTable()
	before := previewCommentTable()
	fullTableComment := `COMMENT='表''注\\行\n次行'`

	t.Run("full", func(t *testing.T) {
		result := GenerateTablePreview(table, TablePreviewFull)
		if result.HasErrors() {
			t.Fatalf("unexpected errors: %+v", result.Diagnostics)
		}
		for _, fragment := range []string{
			"`status` VARCHAR(20) NULL DEFAULT 'O''Brien' COMMENT '状态：字段''注\\\\行\\n次行'",
			"`name_only` INT NULL COMMENT '显示名'",
			"`comment_only` DATE NULL COMMENT '仅备注'",
			"`empty_comment` CHAR(1) NULL COMMENT ''",
			"`named_empty_comment` CHAR(1) NULL COMMENT '空备注名：'",
			"UNIQUE KEY `ix_status` (`status`) COMMENT '索引''注'",
			fullTableComment,
		} {
			if !strings.Contains(result.SQL, fragment) {
				t.Fatalf("full preview missing %q:\n%s", fragment, result.SQL)
			}
		}
		if strings.Contains(result.SQL, "`plain` INT NULL COMMENT") {
			t.Fatalf("full preview emitted a comment for a column without name or comment:\n%s", result.SQL)
		}
	})

	t.Run("standard", func(t *testing.T) {
		result := GenerateTablePreview(table, TablePreviewStandard)
		if result.HasErrors() {
			t.Fatalf("unexpected errors: %+v", result.Diagnostics)
		}
		for _, fragment := range []string{
			"`status` VARCHAR(20) NULL DEFAULT 'O''Brien' COMMENT '字段''注\\\\行\\n次行'",
			"`comment_only` DATE NULL COMMENT '仅备注'",
			"`empty_comment` CHAR(1) NULL COMMENT ''",
			"`named_empty_comment` CHAR(1) NULL COMMENT ''",
		} {
			if !strings.Contains(result.SQL, fragment) {
				t.Fatalf("standard preview missing %q:\n%s", fragment, result.SQL)
			}
		}
		for _, unwanted := range []string{
			"状态：",
			"显示名",
			"索引",
			"表'",
			"COMMENT='",
		} {
			if strings.Contains(result.SQL, unwanted) {
				t.Fatalf("standard preview retained %q:\n%s", unwanted, result.SQL)
			}
		}
		if count := strings.Count(result.SQL, "COMMENT "); count != 4 {
			t.Fatalf("standard preview has %d column-comment clauses, want 4:\n%s", count, result.SQL)
		}
	})

	t.Run("bare", func(t *testing.T) {
		result := GenerateTablePreview(table, TablePreviewBare)
		if result.HasErrors() {
			t.Fatalf("unexpected errors: %+v", result.Diagnostics)
		}
		if strings.Contains(result.SQL, "COMMENT") {
			t.Fatalf("bare preview retained a comment clause:\n%s", result.SQL)
		}
		for _, fragment := range []string{
			"`status` VARCHAR(20) NULL DEFAULT 'O''Brien'",
			"UNIQUE KEY `ix_status` (`status`)",
			") ENGINE=InnoDB;",
		} {
			if !strings.Contains(result.SQL, fragment) {
				t.Fatalf("bare preview missing %q:\n%s", fragment, result.SQL)
			}
		}
	})

	if !reflect.DeepEqual(table, before) {
		t.Fatalf("GenerateTablePreview mutated its source table\nwant: %#v\ngot:  %#v", before, table)
	}
}

func TestGenerateTablePreviewKeepsLegacyGenerateTableOutput(t *testing.T) {
	result := GenerateTable(previewCommentTable())
	if result.HasErrors() {
		t.Fatalf("unexpected errors: %+v", result.Diagnostics)
	}
	if !strings.Contains(result.SQL, "COMMENT '字段''注\\\\行\\n次行'") {
		t.Fatalf("legacy column comment is missing:\n%s", result.SQL)
	}
	if strings.Contains(result.SQL, "状态：") ||
		strings.Contains(result.SQL, "COMMENT '显示名'") {
		t.Fatalf("GenerateTable added preview-only names:\n%s", result.SQL)
	}
	if !strings.Contains(result.SQL, `COMMENT='表''注\\行\n次行'`) ||
		!strings.Contains(result.SQL, "COMMENT '索引''注'") {
		t.Fatalf("GenerateTable lost object comments:\n%s", result.SQL)
	}
}

func TestGenerateTablePreviewUsesTableValidation(t *testing.T) {
	table := previewCommentTable()
	table.Columns[0].DataType = pdm.OptionalText{Value: "UNKNOWN_TYPE", Present: true}

	legacy := GenerateTable(table)
	preview := GenerateTablePreview(table, TablePreviewBare)
	if preview.SQL != "" {
		t.Fatalf("preview emitted SQL for invalid input:\n%s", preview.SQL)
	}
	if !reflect.DeepEqual(preview.Diagnostics, legacy.Diagnostics) {
		t.Fatalf("preview diagnostics differ\nlegacy:  %+v\npreview: %+v", legacy.Diagnostics, preview.Diagnostics)
	}
}

func previewCommentTable() pdm.Table {
	yes, no := true, false
	text := func(value string) pdm.OptionalText {
		return pdm.OptionalText{Value: value, Present: true}
	}
	return pdm.Table{
		Metadata: pdm.Metadata{
			ID:      "table-id",
			Code:    "orders",
			Comment: text("表'注\\行\n次行"),
		},
		Columns: []pdm.Column{
			{
				Metadata: pdm.Metadata{
					ID:      "status-id",
					Code:    "status",
					Name:    "状态",
					Comment: text("字段'注\\行\n次行"),
				},
				DataType:     text("VARCHAR(20)"),
				DefaultValue: text("'O''Brien'"),
				Mandatory:    &no,
			},
			{
				Metadata: pdm.Metadata{
					ID:   "name-only-id",
					Code: "name_only",
					Name: "显示名",
				},
				DataType:  text("INT"),
				Mandatory: &no,
			},
			{
				Metadata: pdm.Metadata{
					ID:      "comment-only-id",
					Code:    "comment_only",
					Comment: text("仅备注"),
				},
				DataType:  text("DATE"),
				Mandatory: &no,
			},
			{
				Metadata: pdm.Metadata{
					ID:      "empty-comment-id",
					Code:    "empty_comment",
					Comment: text(""),
				},
				DataType:  text("CHAR(1)"),
				Mandatory: &no,
			},
			{
				Metadata: pdm.Metadata{
					ID:      "named-empty-comment-id",
					Code:    "named_empty_comment",
					Name:    "空备注名",
					Comment: text(""),
				},
				DataType:  text("CHAR(1)"),
				Mandatory: &no,
			},
			{
				Metadata: pdm.Metadata{
					ID:   "plain-id",
					Code: "plain",
				},
				DataType:  text("INT"),
				Mandatory: &no,
			},
		},
		Indexes: []pdm.Index{
			{
				Metadata: pdm.Metadata{
					ID:      "status-index-id",
					Code:    "ix_status",
					Comment: text("索引'注"),
				},
				Unique: &yes,
				Columns: []pdm.IndexColumn{
					{
						Metadata: pdm.Metadata{ID: "status-index-column-id"},
						ColumnID: "status-id",
					},
				},
			},
		},
	}
}
