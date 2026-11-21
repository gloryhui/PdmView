package appui

import (
	"bytes"

	"pdmview/internal/i18n"
	"pdmview/internal/pdm"
)

type PDMCopyRequest struct {
	sourcePath   string
	sourceXML    []byte
	tableID      string
	edits        []pdm.TextEdit
	activeBoard  int
	writableCopy bool
}

func (r *PDMCopyRequest) SourcePath() string {
	return r.sourcePath
}

func (r *PDMCopyRequest) SourceXML() []byte {
	return r.sourceXML
}

func (r *PDMCopyRequest) WritableCopy() bool {
	return r.writableCopy
}

func (r *PDMCopyRequest) Run() ([]byte, error) {
	return pdm.PatchTableText(r.sourceXML, r.tableID, r.edits)
}

func (r *PDMCopyRequest) Prepare(path string, data []byte) (BoardSet, error) {
	document, err := pdm.Parse(bytes.NewReader(data))
	if err != nil {
		return BoardSet{}, err
	}
	set := buildBoardSet(document, r.activeBoard)
	set.SourcePath, set.SourceXML, set.WritableCopy = path, data, true
	return set, nil
}

func (w *Workspace) SetPDMCopyStarter(starter func(*PDMCopyRequest)) {
	w.pdmCopyStarter = starter
}

func (w *Workspace) PDMCopyCurrent(request *PDMCopyRequest) bool {
	return request != nil && w.pdmCopyRequest == request
}

func (w *Workspace) PublishPDMCopyResult(request *PDMCopyRequest, status i18n.Key, saved *BoardSet) {
	if !w.PDMCopyCurrent(request) {
		return
	}
	w.pdmCopyRequest = nil
	properties := &w.tableProperties
	if properties.tableID == request.tableID {
		properties.copyStatus = status
	}
	if saved != nil {
		w.sourcePath, w.sourceXML, w.writableCopy = saved.SourcePath, saved.SourceXML, true
		w.document, w.boards = saved.Document, saved.Boards
		w.catalog, w.detailIndex, w.searchIndex = saved.Catalog, saved.Details, saved.Search
		prepared := saved.Prepared
		w.canvas.scene = prepared.Graph.Scene
		w.canvas.diagram = prepared.Graph
		w.canvas.index = prepared.Index
		w.canvas.renderer = prepared.Renderer
		w.canvas.prepareVisible()
		w.hasTableDetail = false
		w.refreshSelectedDetail()
		if properties.open {
			properties.detail, properties.hasDetail = w.detailIndex.resolve(properties.tableID)
			if properties.previewEditor == nil || properties.previewEditor.Buffer.String() == properties.previewResult.SQL {
				properties.previewLoaded = false
			}
		}
		w.clearCopyFeedback()
		w.invalidateDatabasePreview()
		w.requestSearch()
	}
	w.notifyChanged()
}

func (w *Workspace) savePDMCopy() {
	properties := &w.tableProperties
	if !properties.open || w.pdmCopyStarter == nil || w.pdmCopyRequest != nil || w.sourcePath == "" || w.loading || w.switching {
		return
	}
	request := &PDMCopyRequest{
		sourcePath:   w.sourcePath,
		sourceXML:    w.sourceXML,
		tableID:      properties.tableID,
		activeBoard:  w.activeBoard,
		writableCopy: w.writableCopy,
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
	w.pdmCopyRequest = request
	properties.copyStatus = i18n.KeyPDMCopySaving
	w.notifyChanged()
	w.pdmCopyStarter(request)
}
