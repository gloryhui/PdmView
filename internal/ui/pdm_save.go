package appui

import (
	"bytes"

	"pdmview/internal/i18n"
	"pdmview/internal/pdm"
)

type PDMSaveRequest struct {
	sourcePath  string
	sourceXML   []byte
	tableID     string
	edits       []pdm.TextEdit
	activeBoard int
}

func (r *PDMSaveRequest) SourcePath() string {
	return r.sourcePath
}

func (r *PDMSaveRequest) SourceXML() []byte {
	return r.sourceXML
}

func (r *PDMSaveRequest) Run() ([]byte, error) {
	return pdm.PatchTableText(r.sourceXML, r.tableID, r.edits)
}

func (r *PDMSaveRequest) Prepare(data []byte) (BoardSet, error) {
	document, err := pdm.Parse(bytes.NewReader(data))
	if err != nil {
		return BoardSet{}, err
	}
	set := buildBoardSet(document, r.activeBoard)
	set.SourcePath, set.SourceXML = r.sourcePath, data
	return set, nil
}

func (w *Workspace) SetPDMSaveStarter(starter func(*PDMSaveRequest)) {
	w.pdmSaveStarter = starter
}

func (w *Workspace) PDMSaveCurrent(request *PDMSaveRequest) bool {
	return request != nil && w.pdmSaveRequest == request
}

func (w *Workspace) PublishPDMSaveResult(request *PDMSaveRequest, status i18n.Key, saved *BoardSet) {
	if !w.PDMSaveCurrent(request) {
		return
	}
	w.pdmSaveRequest = nil
	properties := &w.tableProperties
	if properties.tableID == request.tableID {
		properties.saveStatus = status
	}
	if saved != nil {
		w.sourceXML = saved.SourceXML
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

func (w *Workspace) savePDM() {
	properties := &w.tableProperties
	if !properties.open || w.pdmSaveStarter == nil || w.pdmSaveRequest != nil || w.sourcePath == "" || w.loading || w.switching {
		return
	}
	request := &PDMSaveRequest{
		sourcePath:  w.sourcePath,
		sourceXML:   w.sourceXML,
		tableID:     properties.tableID,
		activeBoard: w.activeBoard,
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
	w.pdmSaveRequest = request
	properties.saveStatus = i18n.KeyPDMSaving
	w.notifyChanged()
	w.pdmSaveStarter(request)
}
