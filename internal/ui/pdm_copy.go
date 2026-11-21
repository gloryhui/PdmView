package appui

import (
	"pdmview/internal/i18n"
	"pdmview/internal/pdm"
)

type PDMCopyRequest struct {
	sourcePath string
	sourceXML  []byte
	tableID    string
	edits      []pdm.TextEdit
}

func (r *PDMCopyRequest) SourcePath() string {
	return r.sourcePath
}

func (r *PDMCopyRequest) Run() ([]byte, error) {
	return pdm.PatchTableText(r.sourceXML, r.tableID, r.edits)
}

func (w *Workspace) SetPDMCopyStarter(starter func(*PDMCopyRequest)) {
	w.pdmCopyStarter = starter
}

func (w *Workspace) PDMCopyCurrent(request *PDMCopyRequest) bool {
	return request != nil && w.tableProperties.copyRequest == request
}

func (w *Workspace) PublishPDMCopyResult(request *PDMCopyRequest, status i18n.Key) {
	if !w.PDMCopyCurrent(request) {
		return
	}
	w.tableProperties.copyRequest = nil
	w.tableProperties.copyStatus = status
	w.notifyChanged()
}

func (w *Workspace) savePDMCopy() {
	properties := &w.tableProperties
	if !properties.open || w.pdmCopyStarter == nil || properties.copyRequest != nil || w.sourcePath == "" {
		return
	}
	request := &PDMCopyRequest{
		sourcePath: w.sourcePath,
		sourceXML:  w.sourceXML,
		tableID:    properties.tableID,
	}
	collect := func(original pdm.Metadata, name, code, comment string) {
		if name != original.Name {
			request.edits = append(request.edits, pdm.TextEdit{ObjectID: original.ID, Property: pdm.TextName, Value: name})
		}
		if code != original.Code {
			request.edits = append(request.edits, pdm.TextEdit{ObjectID: original.ID, Property: pdm.TextCode, Value: code})
		}
		if comment != original.Comment.Value {
			request.edits = append(request.edits, pdm.TextEdit{ObjectID: original.ID, Property: pdm.TextComment, Value: comment})
		}
	}
	detail, draft := &properties.detail, &properties.draft
	collect(detail.Metadata, draft.General.Name, draft.General.Code, draft.General.Comment)
	for index, column := range draft.Columns {
		collect(detail.Columns[index].Metadata, column.Name, column.Code, column.Comment)
	}
	properties.copyRequest = request
	properties.copyStatus = i18n.KeyPDMCopySaving
	w.notifyChanged()
	w.pdmCopyStarter(request)
}
