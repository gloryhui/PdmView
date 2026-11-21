package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"pdmview/internal/pdm"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "pdm-inspect: usage: pdm-inspect <file.pdm>")
		return 2
	}
	report := inspectPath(args[0])
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(stderr, "pdm-inspect: report write failed")
		return 1
	}
	if report.Fatal != nil {
		return 1
	}
	return 0
}

func inspectPath(path string) Report {
	info, err := os.Stat(path)
	if err != nil {
		return fatalReport("stat-failed")
	}
	if !info.Mode().IsRegular() {
		return fatalReport("not-regular-file")
	}

	file, err := os.Open(path)
	if err != nil {
		return fatalReport("open-failed")
	}
	defer file.Close()

	info, err = file.Stat()
	if err != nil {
		return fatalReport("stat-failed")
	}
	if !info.Mode().IsRegular() {
		return fatalReport("not-regular-file")
	}
	if info.Size() > pdm.MaxDocumentSize {
		return fatalReport("document-too-large")
	}

	document, err := pdm.Parse(file)
	if err != nil {
		return fatalReport(fatalCode(err))
	}
	return BuildReport(document)
}

func fatalCode(err error) string {
	switch {
	case errors.Is(err, pdm.ErrMalformedXML):
		return "malformed-xml"
	case errors.Is(err, pdm.ErrUnsupportedEncoding):
		return "unsupported-encoding"
	case errors.Is(err, pdm.ErrUnsupportedDirective):
		return "unsupported-directive"
	case errors.Is(err, pdm.ErrDocumentTooLarge):
		return "document-too-large"
	case errors.Is(err, pdm.ErrDepthLimit):
		return "depth-limit"
	case errors.Is(err, pdm.ErrElementLimit):
		return "element-limit"
	case errors.Is(err, pdm.ErrMultipleRoots):
		return "multiple-roots"
	case errors.Is(err, pdm.ErrInvalidStructure):
		return "invalid-structure"
	case errors.Is(err, pdm.ErrForeignSignature):
		return "foreign-signature"
	case errors.Is(err, pdm.ErrReadFailure):
		return "read-failure"
	default:
		return "parse-error"
	}
}

// Report is the stable JSON document emitted by pdm-inspect.
type Report struct {
	Format         FormatReport       `json:"format"`
	RawDefinitions int                `json:"raw_definitions"`
	Summary        SummaryReport      `json:"summary"`
	Diagnostics    []DiagnosticReport `json:"diagnostics"`
	Fatal          *FatalReport       `json:"fatal,omitempty"`
}

// FormatReport summarizes only safe writer and XML envelope fields.
type FormatReport struct {
	Signature     string `json:"signature"`
	WriterVersion string `json:"writer_version,omitempty"`
	XMLVersion    string `json:"xml_version"`
	Encoding      string `json:"encoding"`
}

// SummaryReport is the recursive typed-object inventory.
type SummaryReport struct {
	Packages         int `json:"packages"`
	Tables           int `json:"tables"`
	Columns          int `json:"columns"`
	Keys             int `json:"keys"`
	PrimaryKeys      int `json:"primary_keys"`
	Indexes          int `json:"indexes"`
	References       int `json:"references"`
	Joins            int `json:"joins"`
	Diagrams         int `json:"diagrams"`
	TableSymbols     int `json:"table_symbols"`
	ReferenceSymbols int `json:"reference_symbols"`
	PathPoints       int `json:"path_points"`
}

// DiagnosticReport contains only the stable diagnostic identity.
type DiagnosticReport struct {
	Code string `json:"code"`
	Path string `json:"path"`
	Line int    `json:"line"`
}

// FatalReport intentionally carries no source text, IDs, or file paths.
type FatalReport struct {
	Code string `json:"code"`
}

// BuildReport converts a parsed PDM document into the CLI JSON report.
func BuildReport(document *pdm.Document) Report {
	report := Report{Diagnostics: []DiagnosticReport{}}
	if document == nil || document.Raw == nil {
		report.Fatal = &FatalReport{Code: "parse-error"}
		return report
	}
	report.Format = FormatReport{
		Signature:     document.Raw.Format.Signature,
		WriterVersion: numericVersion(document.Raw.Format.PowerDesignerVersion),
		XMLVersion:    document.Raw.Format.XMLVersion,
		Encoding:      document.Raw.Format.Encoding,
	}
	if document.Raw.Root != nil {
		report.RawDefinitions = countDefinitions(document.Raw.Root)
	}
	report.Summary = summarize(document.Model.Contents)
	report.Diagnostics = make([]DiagnosticReport, 0, len(document.Diagnostics))
	for _, diagnostic := range document.Diagnostics {
		report.Diagnostics = append(report.Diagnostics, DiagnosticReport{
			Code: diagnostic.Code,
			Path: diagnostic.Path,
			Line: diagnostic.Line,
		})
	}
	return report
}

func fatalReport(code string) Report {
	return Report{
		Diagnostics: []DiagnosticReport{},
		Fatal:       &FatalReport{Code: code},
	}
}

func numericVersion(version string) string {
	if version == "" || len(version) > 64 {
		return ""
	}
	segmentStart := 0
	for index := 0; index <= len(version); index++ {
		if index < len(version) && version[index] >= '0' && version[index] <= '9' {
			continue
		}
		if index == len(version) || version[index] != '.' {
			if index == len(version) && segmentStart == index {
				return ""
			}
			if index < len(version) {
				return ""
			}
			break
		}
		if segmentStart == index {
			return ""
		}
		segmentStart = index + 1
	}
	return version
}

func countDefinitions(element *pdm.Element) int {
	if element == nil {
		return 0
	}
	count := 0
	if element.Name.Space == pdm.NamespaceObject {
		if id, present := element.Attribute("Id"); present && id != "" {
			count++
		}
	}
	for _, child := range element.Children {
		count += countDefinitions(child)
	}
	return count
}

func summarize(contents pdm.Contents) SummaryReport {
	var summary SummaryReport
	addContents(contents, &summary)
	return summary
}

func addContents(contents pdm.Contents, summary *SummaryReport) {
	summary.Packages += len(contents.Packages)
	summary.Tables += len(contents.Tables)
	summary.References += len(contents.References)
	summary.Diagrams += len(contents.Diagrams)

	for _, table := range contents.Tables {
		summary.Columns += len(table.Columns)
		summary.Keys += len(table.Keys)
		if table.PrimaryKeyID != "" {
			summary.PrimaryKeys++
		}
		summary.Indexes += len(table.Indexes)
	}
	for _, reference := range contents.References {
		summary.Joins += len(reference.Joins)
	}
	for _, diagram := range contents.Diagrams {
		summary.TableSymbols += len(diagram.TableSymbols)
		summary.ReferenceSymbols += len(diagram.ReferenceSymbols)
		for _, symbol := range diagram.ReferenceSymbols {
			summary.PathPoints += len(symbol.Points)
		}
	}
	for _, pkg := range contents.Packages {
		addContents(pkg.Contents, summary)
	}
}
