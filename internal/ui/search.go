package appui

import (
	"context"
	"strings"

	"pdmview/internal/pdm"

	"github.com/egoist/mygo/ui"
)

const (
	SearchKey        = "pdmview.workspace.search"
	SearchResultsKey = "pdmview.workspace.search.results"

	searchCancelCheckInterval = 64
)

// SearchResult identifies one distinct table with a matching searchable term.
type SearchResult struct {
	TableID string
	Name    string
}

// SearchResultKey returns the stable result key for a table.
func SearchResultKey(tableID string) string {
	return "pdmview.workspace.search.result." + tableID
}

type searchTable struct {
	id    string
	name  string
	terms []string
}

type searchIndex struct {
	tables []searchTable
}

type searchIndexBuilder struct {
	index searchIndex
	byID  map[string]int
}

func buildSearchIndex(model *pdm.Model) *searchIndex {
	builder := searchIndexBuilder{
		byID: make(map[string]int),
	}
	if model != nil {
		builder.collect(model.Contents)
	}
	return &builder.index
}

func (builder *searchIndexBuilder) collect(contents pdm.Contents) {
	for _, table := range contents.Tables {
		if table.ID == "" {
			continue
		}
		terms := tableSearchTerms(table)
		if offset, ok := builder.byID[table.ID]; ok {
			builder.index.tables[offset].terms = appendUniqueTerms(
				builder.index.tables[offset].terms,
				terms,
			)
			continue
		}
		builder.byID[table.ID] = len(builder.index.tables)
		builder.index.tables = append(builder.index.tables, searchTable{
			id:    table.ID,
			name:  firstNonEmpty(table.Name, table.Code, table.ID),
			terms: terms,
		})
	}
	for _, pkg := range contents.Packages {
		builder.collect(pkg.Contents)
	}
}

func tableSearchTerms(table pdm.Table) []string {
	terms := make([]string, 0, 3+len(table.Columns)*3)
	terms = appendSearchTerm(terms, table.Name)
	terms = appendSearchTerm(terms, table.Code)
	terms = appendSearchTerm(terms, table.Comment.Value)
	for _, column := range table.Columns {
		terms = appendSearchTerm(terms, column.Name)
		terms = appendSearchTerm(terms, column.Code)
		terms = appendSearchTerm(terms, column.Comment.Value)
	}
	return terms
}

func appendSearchTerm(terms []string, value string) []string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return terms
	}
	return append(terms, value)
}

func appendUniqueTerms(terms, extra []string) []string {
	for _, value := range extra {
		found := false
		for _, existing := range terms {
			if existing == value {
				found = true
				break
			}
		}
		if !found {
			terms = append(terms, value)
		}
	}
	return terms
}

func (index *searchIndex) search(ctx context.Context, query string) []SearchResult {
	query = strings.ToLower(strings.TrimSpace(query))
	if index == nil || query == "" || len(index.tables) == 0 {
		return nil
	}
	results := make([]SearchResult, 0)
	for offset := range index.tables {
		if offset%searchCancelCheckInterval == 0 && contextCanceled(ctx) {
			return nil
		}
		table := &index.tables[offset]
		for termOffset, term := range table.terms {
			if termOffset%searchCancelCheckInterval == 0 && contextCanceled(ctx) {
				return nil
			}
			if strings.Contains(term, query) {
				results = append(results, SearchResult{TableID: table.id, Name: table.name})
				break
			}
		}
	}
	if contextCanceled(ctx) {
		return nil
	}
	return results
}

func contextCanceled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// SearchRequest is one cancelable search over an immutable document snapshot.
type SearchRequest struct {
	workspace     *Workspace
	index         *searchIndex
	ctx           context.Context
	cancel        func()
	query         string
	token         uint64
	documentToken uint64
}

// Run performs the search off the UI thread. A canceled or stale request
// returns nil; publishing, rather than running, decides which result shows.
func (request SearchRequest) Run() []SearchResult {
	return request.index.search(request.ctx, request.query)
}

func (w *Workspace) SetSearchStarter(starter func(SearchRequest)) {
	w.searchStarter = starter
}

// BeginSearch starts a new query and invalidates every earlier request.
func (w *Workspace) BeginSearch(query string) (SearchRequest, bool) {
	w.cancelSearch()
	w.searchQuery = query
	w.searchResults = nil
	w.searchFinished = false
	w.searchSelected = -1
	w.searchList.ScrollTo(0, ui.Start)
	w.searchToken++
	if strings.TrimSpace(query) == "" {
		w.notifyChanged()
		return SearchRequest{}, false
	}
	if w.searchIndex == nil {
		w.searchFinished = true
		w.notifyChanged()
		return SearchRequest{}, false
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.searchCancel = cancel
	request := SearchRequest{
		workspace:     w,
		index:         w.searchIndex,
		ctx:           ctx,
		cancel:        cancel,
		query:         query,
		token:         w.searchToken,
		documentToken: w.loadToken,
	}
	w.notifyChanged()
	return request, true
}

// PublishSearchResults adopts a completed result only for the live document
// and query.
func (w *Workspace) PublishSearchResults(request SearchRequest, results []SearchResult) bool {
	if request.workspace != w ||
		request.token != w.searchToken ||
		request.documentToken != w.loadToken ||
		request.query != w.searchQuery {
		if request.cancel != nil {
			request.cancel()
		}
		return false
	}
	if w.searchCancel != nil {
		w.searchCancel()
		w.searchCancel = nil
	}
	w.searchResults = append([]SearchResult(nil), results...)
	w.searchFinished = true
	w.syncSearchSelected()
	w.searchList.ScrollTo(0, ui.Start)
	w.notifyChanged()
	return true
}

func (w *Workspace) requestSearch() {
	request, ok := w.BeginSearch(w.searchQuery)
	if !ok {
		return
	}
	if w.searchStarter != nil {
		w.searchStarter(request)
		return
	}
	w.PublishSearchResults(request, request.Run())
}

func (w *Workspace) cancelSearch() {
	if w.searchCancel != nil {
		w.searchCancel()
		w.searchCancel = nil
	}
}

func (w *Workspace) resetSearch() {
	w.cancelSearch()
	w.searchToken++
	w.searchQuery = ""
	w.searchResults = nil
	w.searchFinished = false
	w.searchSelected = -1
	w.searchList.ScrollTo(0, ui.Start)
}

func (w *Workspace) searchActive() bool {
	return strings.TrimSpace(w.searchQuery) != ""
}

func (w *Workspace) syncSearchSelected() {
	w.searchSelected = -1
	for offset := range w.searchResults {
		if w.searchResults[offset].TableID == w.selectedTableID {
			w.searchSelected = offset
			return
		}
	}
}
