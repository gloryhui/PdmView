package pdm

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

const copyFixture = `<?xml version="1.0" encoding="UTF-8"?>
<?PowerDesigner version="16.5.0.3982" signature="PDM_DATA_MODEL_XML"?>
<!-- file header -->
<Model xmlns:d="attribute" xmlns:c="collection" xmlns:o="object" xmlns:ext="extension">
<o:RootObject Id="root"><c:Children><o:Model Id="model"><c:Tables>
<o:Table Id="table">
  <d:Name>Orders</d:Name>
  <d:Code>ORDERS</d:Code>
  <!-- table note -->
  <c:Columns>
    <o:Column Id="id"><d:Name>ID</d:Name><d:Code>ID</d:Code><d:Comment /></o:Column>
    <o:Column Id="amount"><d:Name>Amount</d:Name><d:Code>AMOUNT</d:Code><d:DataType>int(20)</d:DataType></o:Column>
  </c:Columns>
  <c:Keys><o:Key Id="key"><c:Key.Columns><o:Column Ref="id"/></c:Key.Columns></o:Key></c:Keys>
  <c:PrimaryKey><o:Key Ref="key"/></c:PrimaryKey>
  <ext:Vendor attr='keep'>&#x41;</ext:Vendor>
</o:Table>
<o:Table Id="other">
  <d:Code>OTHER</d:Code>
  <c:Columns><o:Column Id="foreign"><d:Code>ID</d:Code></o:Column></c:Columns>
</o:Table>
</c:Tables></o:Model></c:Children></o:RootObject>
</Model>
`

func TestPatchTableTextExactBytes(t *testing.T) {
	withEnvelope := func(text string) []byte {
		return []byte("\ufeff" + strings.ReplaceAll(text, "\n", "\r\n"))
	}
	source := withEnvelope(copyFixture)
	for _, edits := range [][]TextEdit{nil, {
		{ObjectID: "table", Property: TextName, Value: "Orders"},
		{ObjectID: "table", Property: TextComment, Value: ""},
	}} {
		got, err := PatchTableText(source, "table", edits)
		if err != nil || !bytes.Equal(got, source) {
			t.Fatal("unedited copy differs from the original", err)
		}
	}
	got, err := PatchTableText(source, "table", []TextEdit{
		{ObjectID: "table", Property: TextName, Value: "订单 <&>"},
		{ObjectID: "table", Property: TextCode, Value: "ORDERS_NEW"},
		{ObjectID: "table", Property: TextComment, Value: "新注释"},
		{ObjectID: "id", Property: TextComment, Value: "首行\n次行"},
		{ObjectID: "amount", Property: TextCode, Value: "AMOUNT_NEW"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.NewReplacer(
		"<d:Name>Orders</d:Name>", "<d:Name>订单 &lt;&amp;&gt;</d:Name>",
		"<d:Code>ORDERS</d:Code>", "<d:Code>ORDERS_NEW</d:Code>",
		"<c:Columns>\n", "<d:Comment>新注释</d:Comment>\n  <c:Columns>\n",
		"<d:Comment />", "<d:Comment >首行&#xA;次行</d:Comment>",
		"<d:Code>AMOUNT</d:Code>", "<d:Code>AMOUNT_NEW</d:Code>",
	).Replace(copyFixture)
	if !bytes.Equal(got, withEnvelope(want)) {
		t.Fatalf("bytes outside the expected edits changed:\n%s", got)
	}
	raw, err := Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := raw.ByID["id"].Property("Comment"); value != "首行\n次行" {
		t.Fatalf("decoded comment = %q", value)
	}
}

func TestPatchTableTextRejectsInvalidTargetsAndValues(t *testing.T) {
	for _, edits := range [][]TextEdit{
		{{ObjectID: "other", Property: TextName, Value: "New"}},
		{{ObjectID: "foreign", Property: TextName, Value: "New"}},
		{{ObjectID: "key", Property: TextName, Value: "New"}},
		{{ObjectID: "amount", Property: "DataType", Value: "text"}},
		{{ObjectID: "amount", Property: TextName, Value: " "}},
		{{ObjectID: "table", Property: TextComment, Value: "\x00bad"}},
		{{ObjectID: "table", Property: TextComment, Value: "\xff"}},
		{{ObjectID: "table", Property: TextCode, Value: "other"}},
		{{ObjectID: "amount", Property: TextCode, Value: "id"}},
		{
			{ObjectID: "id", Property: TextName, Value: "First"},
			{ObjectID: "id", Property: TextName, Value: "Second"},
		},
	} {
		if data, err := PatchTableText([]byte(copyFixture), "table", edits); !errors.Is(err, ErrInvalidTextEdit) || data != nil {
			t.Fatalf("accepted invalid edit %v: %v", edits, err)
		}
	}
}

func TestPatchTableTextPreservesEmbeddedMarkupAndAmbiguousObjects(t *testing.T) {
	for _, source := range []string{
		strings.Replace(copyFixture, "<d:Comment />", "<d:Comment><!-- keep --></d:Comment>", 1),
		strings.Replace(copyFixture, "<d:Comment />", "<d:Comment><ext:Note/></d:Comment>", 1),
		strings.Replace(copyFixture, "<d:Comment />", "<d:Comment/><d:Comment/>", 1),
		strings.Replace(copyFixture, `Id="amount"`, `Id="id"`, 1),
	} {
		if _, err := PatchTableText([]byte(source), "table", []TextEdit{
			{ObjectID: "id", Property: TextComment, Value: "Edit"},
		}); !errors.Is(err, ErrInvalidTextEdit) {
			t.Fatalf("expected edit refusal, got %v", err)
		}
	}
}
