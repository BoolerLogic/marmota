package bridge

import (
	"strings"
	"sync"
)

// #6 Que es una Entry:
// #8 Es HTTPHistoryEntryDetail, una estructura que contiene una Request (Siempre) y una Response (Solo cuando llega, si es que llega)

// #6 De que se compone un Filtro:
// #8 - ID: Identificador único asociado a la Pestaña creada en el Frontend.
// #8 - Version: Al crear un filtro, en la misma pestaña podemos modificarlo, al ser modificado, aumento en +1 la Versión.
// #8 - Condiciones y Operador para evaluar las mismas.

// #6 Cuando el Frontend añade o modifica un filtro:
// #8 1.- El frontend llama al backend y este ejecuta UpsertActiveHistoryFilter(...).
// #8     El backend guarda o actualiza ese filtro en memoria y devuelve la lista de IDs
// #8     de las entries del historial que cumplen ese filtro en ese momento.
// #8 2.- Mientras esa llamada a UpsertActiveHistoryFilter(...) está en vuelo,
// #8     el frontend guarda en un Set los IDs de las entries que van entrando o actualizándose.
// #8 3.- Cuando el backend devuelve los matching IDs iniciales del filtro,
// #8     el frontend envía ese Set al backend mediante GetHistoryFilterMatchesForEntries(...).
// #8 4.- El backend devuelve, para cada ID de ese Set, qué filtros activos cumple actualmente.
// #8 5.- El frontend aplica esos resultados solo si el filterId y la version coinciden
// #8     con la versión actual de cada pestaña filtrada.

// #6 Una vez añadido o modificado un filtro:
// #8 1.- Si entra una nueva request o response al backend, este actualiza primero
// #8     la entry correspondiente en memoria.
// #8 2.- Después evalúa esa entry actualizada contra los filtros activos en memoria.
// #8 3.- Luego envía la entry al frontend junto con la lista de filtros
// #8     que cumple actualmente esa entry, incluyendo filterId y version.

// #6 Cuando el Frontend elimina un filtro:
// #8 1.- El frontend llama al backend y este ejecuta RemoveActiveHistoryFilter(...),
// #8     eliminando ese filtro de la memoria del backend solo si la version coincide.

type HistoryFilterTarget string

const (
	HistoryFilterTargetAll          HistoryFilterTarget = "all"
	HistoryFilterTargetRequest      HistoryFilterTarget = "request"
	HistoryFilterTargetResponse     HistoryFilterTarget = "response"
	HistoryFilterTargetRequestHead  HistoryFilterTarget = "requestHead"
	HistoryFilterTargetResponseHead HistoryFilterTarget = "responseHead"
	HistoryFilterTargetRequestBody  HistoryFilterTarget = "requestBody"
	HistoryFilterTargetResponseBody HistoryFilterTarget = "responseBody"
	HistoryFilterTargetHead         HistoryFilterTarget = "head"
	HistoryFilterTargetBody         HistoryFilterTarget = "body"
	HistoryFilterTargetMethod       HistoryFilterTarget = "method"
	HistoryFilterTargetHost         HistoryFilterTarget = "host"
	HistoryFilterTargetPort         HistoryFilterTarget = "port"
	HistoryFilterTargetScheme       HistoryFilterTarget = "scheme"
	HistoryFilterTargetPath         HistoryFilterTarget = "path"
)

type HistoryFilterMatchMode string

const (
	HistoryMatchContains    HistoryFilterMatchMode = "contains"
	HistoryMatchNotContains HistoryFilterMatchMode = "notContains"
	HistoryMatchEquals      HistoryFilterMatchMode = "equals"
	HistoryMatchNotEquals   HistoryFilterMatchMode = "notEquals"
	HistoryMatchStartsWith  HistoryFilterMatchMode = "startsWith"
	HistoryMatchEndsWith    HistoryFilterMatchMode = "endsWith"
)

type HistoryFilterOperator string

const (
	HistoryFilterAnd HistoryFilterOperator = "and"
	HistoryFilterOr  HistoryFilterOperator = "or"
)

type HistoryFilterCondition struct {
	Query     string                 `json:"query"`
	Target    HistoryFilterTarget    `json:"target"`
	MatchMode HistoryFilterMatchMode `json:"matchMode"`
}

type UpsertActiveHistoryFilterParams struct {
	FilterID   string                   `json:"filterId"`
	Version    uint64                   `json:"version"`
	Conditions []HistoryFilterCondition `json:"conditions"`
	Operator   HistoryFilterOperator    `json:"operator"`
}

type UpsertActiveHistoryFilterResult struct {
	FilterID    string   `json:"filterId"`
	Version     uint64   `json:"version"`
	MatchingIDs []uint64 `json:"matchingIds"`
}

type RemoveActiveHistoryFilterParams struct {
	FilterID string `json:"filterId"`
	Version  uint64 `json:"version"`
}

type GetHistoryFilterMatchesForEntriesParams struct {
	EntryIDs []uint64 `json:"entryIds"`
}

type HistoryFilterMatch struct {
	FilterID string `json:"filterId"`
	Version  uint64 `json:"version"`
}

type HistoryEntryFilterMatches struct {
	EntryID uint64               `json:"entryId"`
	Matches []HistoryFilterMatch `json:"matches"`
}

type compiledHistoryFilterCondition struct {
	query     string
	target    HistoryFilterTarget
	matchMode HistoryFilterMatchMode
}

type activeHistoryFilter struct {
	filterID   string
	version    uint64
	operator   HistoryFilterOperator
	conditions []compiledHistoryFilterCondition
}

var activeHistoryFilters = make(map[string]activeHistoryFilter, 8)
var activeHistoryFiltersMu sync.RWMutex

func UpsertActiveHistoryFilter(params UpsertActiveHistoryFilterParams) UpsertActiveHistoryFilterResult {
	filterID := strings.TrimSpace(params.FilterID)
	if filterID == "" {
		return UpsertActiveHistoryFilterResult{}
	}

	nextFilter := activeHistoryFilter{
		filterID:   filterID,
		version:    params.Version,
		operator:   normalizeHistoryFilterOperator(params.Operator),
		conditions: compileHistoryFilterConditions(params.Conditions),
	}

	activeHistoryFiltersMu.Lock()
	currentFilter, exists := activeHistoryFilters[filterID]

	// No dejamos que una version vieja machaque una nueva.
	if exists && currentFilter.version > nextFilter.version {
		nextFilter = currentFilter
	} else {
		activeHistoryFilters[filterID] = nextFilter
	}
	activeHistoryFiltersMu.Unlock()

	matchingIDs := collectMatchingHistoryIDs(nextFilter)

	return UpsertActiveHistoryFilterResult{
		FilterID:    nextFilter.filterID,
		Version:     nextFilter.version,
		MatchingIDs: matchingIDs,
	}
}

func RemoveActiveHistoryFilter(params RemoveActiveHistoryFilterParams) {
	filterID := strings.TrimSpace(params.FilterID)
	if filterID == "" {
		return
	}

	activeHistoryFiltersMu.Lock()
	defer activeHistoryFiltersMu.Unlock()

	currentFilter, exists := activeHistoryFilters[filterID]
	if !exists {
		return
	}

	// Solo borramos si la version coincide exactamente con la activa.
	if currentFilter.version != params.Version {
		return
	}

	delete(activeHistoryFilters, filterID)
}

func ClearActiveHistoryFilters() {
	activeHistoryFiltersMu.Lock()
	clear(activeHistoryFilters)
	activeHistoryFiltersMu.Unlock()
}

func GetHistoryFilterMatchesForEntries(params GetHistoryFilterMatchesForEntriesParams) []HistoryEntryFilterMatches {
	entryIDs := uniqueUint64s(params.EntryIDs)
	if len(entryIDs) == 0 {
		return []HistoryEntryFilterMatches{}
	}

	activeFilters := snapshotActiveHistoryFilters()
	if len(activeFilters) == 0 {
		results := make([]HistoryEntryFilterMatches, 0, len(entryIDs))
		for _, entryID := range entryIDs {
			results = append(results, HistoryEntryFilterMatches{
				EntryID: entryID,
				Matches: []HistoryFilterMatch{},
			})
		}
		return results
	}

	historyEntries := snapshotHistoryEntries(entryIDs)

	results := make([]HistoryEntryFilterMatches, 0, len(entryIDs))
	for _, entryID := range entryIDs {
		entry := historyEntries[entryID]
		if entry == nil {
			results = append(results, HistoryEntryFilterMatches{
				EntryID: entryID,
				Matches: []HistoryFilterMatch{},
			})
			continue
		}

		candidateCache := make(map[HistoryFilterTarget]string, 8)
		matches := make([]HistoryFilterMatch, 0, len(activeFilters))

		for _, filter := range activeFilters {
			if entryMatchesCompiledHistoryFilters(entry, filter, candidateCache) {
				matches = append(matches, HistoryFilterMatch{
					FilterID: filter.filterID,
					Version:  filter.version,
				})
			}
		}

		results = append(results, HistoryEntryFilterMatches{
			EntryID: entryID,
			Matches: matches,
		})
	}

	return results
}

func collectMatchingHistoryIDs(filter activeHistoryFilter) []uint64 {
	historyMu.RLock()
	defer historyMu.RUnlock()

	if len(history) == 0 {
		return []uint64{}
	}

	matchingIDs := make([]uint64, 0, len(history))
	for _, entry := range history {
		if entry == nil {
			continue
		}
		// #8 "candidateCache" guarda en Cache los textos en minusculas y concatenados que usemos en una condicion. Para no tener que volver a procesarlos si lo usamos en otra condición.
		candidateCache := make(map[HistoryFilterTarget]string, 4)
		if entryMatchesCompiledHistoryFilters(entry, filter, candidateCache) {
			matchingIDs = append(matchingIDs, entry.ID)
		}
	}

	return matchingIDs
}

func snapshotActiveHistoryFilters() []activeHistoryFilter {
	activeHistoryFiltersMu.RLock()
	defer activeHistoryFiltersMu.RUnlock()

	if len(activeHistoryFilters) == 0 {
		return []activeHistoryFilter{}
	}

	filters := make([]activeHistoryFilter, 0, len(activeHistoryFilters))
	for _, filter := range activeHistoryFilters {
		filters = append(filters, filter)
	}
	return filters
}

func snapshotHistoryEntries(entryIDs []uint64) map[uint64]*HTTPHistoryEntryDetail {
	historyMu.RLock()
	defer historyMu.RUnlock()

	entries := make(map[uint64]*HTTPHistoryEntryDetail, len(entryIDs))
	for _, entryID := range entryIDs {
		if entry, exists := history[entryID]; exists && entry != nil {
			entryCopy := *entry
			if entry.Request != nil {
				requestCopy := *entry.Request
				entryCopy.Request = &requestCopy
			}
			if entry.Response != nil {
				responseCopy := *entry.Response
				entryCopy.Response = &responseCopy
			}
			entries[entryID] = &entryCopy
		}
	}

	return entries
}

func entryMatchesCompiledHistoryFilters(
	entry *HTTPHistoryEntryDetail,
	filter activeHistoryFilter,
	candidateCache map[HistoryFilterTarget]string,
) bool {
	if len(filter.conditions) == 0 {
		return true
	}

	if filter.operator == HistoryFilterOr {
		for _, condition := range filter.conditions {
			candidate, ok := candidateCache[condition.target]
			if !ok {
				candidate = buildHistoryFilterCandidate(entry, condition.target)
				candidateCache[condition.target] = candidate
			}

			if matchesHistoryFilterCondition(candidate, condition) {
				return true
			}
		}
		return false
	}

	for _, condition := range filter.conditions {
		candidate, ok := candidateCache[condition.target]
		if !ok {
			candidate = buildHistoryFilterCandidate(entry, condition.target)
			candidateCache[condition.target] = candidate
		}

		if !matchesHistoryFilterCondition(candidate, condition) {
			return false
		}
	}

	return true
}

func compileHistoryFilterConditions(
	conditions []HistoryFilterCondition,
) []compiledHistoryFilterCondition {
	compiled := make([]compiledHistoryFilterCondition, 0, len(conditions))

	for _, condition := range conditions {
		query := strings.TrimSpace(condition.Query)
		if query == "" {
			continue
		}

		compiled = append(compiled, compiledHistoryFilterCondition{
			query:     normalizeSearchableText(query),
			target:    normalizeHistoryFilterTarget(condition.Target),
			matchMode: normalizeHistoryFilterMatchMode(condition.MatchMode),
		})
	}

	return compiled
}

func matchesHistoryFilterCondition(
	candidate string,
	condition compiledHistoryFilterCondition,
) bool {
	switch condition.matchMode {
	case HistoryMatchContains:
		return strings.Contains(candidate, condition.query)
	case HistoryMatchNotContains:
		return !strings.Contains(candidate, condition.query)
	case HistoryMatchEquals:
		return candidate == condition.query
	case HistoryMatchNotEquals:
		return candidate != condition.query
	case HistoryMatchStartsWith:
		return strings.HasPrefix(candidate, condition.query)
	case HistoryMatchEndsWith:
		return strings.HasSuffix(candidate, condition.query)
	default:
		panic("HistoryFilterMatchMode no configurado al Filtrar Peticiones")
	}
}

func buildHistoryFilterCandidate(
	entry *HTTPHistoryEntryDetail,
	target HistoryFilterTarget,
) string {
	requestHead := getRequestHead(entry)
	requestBody := getRequestBody(entry)
	responseHead := getResponseHead(entry)
	responseBody := getResponseBody(entry)

	switch target {
	case HistoryFilterTargetAll:
		return normalizeSearchableText(joinNonEmpty(
			requestHead,
			requestBody,
			responseHead,
			responseBody,
		))
	case HistoryFilterTargetRequest:
		return normalizeSearchableText(joinNonEmpty(requestHead, requestBody))
	case HistoryFilterTargetResponse:
		return normalizeSearchableText(joinNonEmpty(responseHead, responseBody))
	case HistoryFilterTargetRequestHead:
		return normalizeSearchableText(requestHead)
	case HistoryFilterTargetResponseHead:
		return normalizeSearchableText(responseHead)
	case HistoryFilterTargetRequestBody:
		return normalizeSearchableText(requestBody)
	case HistoryFilterTargetResponseBody:
		return normalizeSearchableText(responseBody)
	case HistoryFilterTargetHead:
		return normalizeSearchableText(joinNonEmpty(requestHead, responseHead))
	case HistoryFilterTargetBody:
		return normalizeSearchableText(joinNonEmpty(requestBody, responseBody))
	case HistoryFilterTargetMethod:
		return normalizeSearchableText(getRequestMethod(entry))
	case HistoryFilterTargetHost:
		return normalizeSearchableText(joinDistinctNonEmpty(
			getRequestHost(entry),
			getResponseHost(entry),
		))
	case HistoryFilterTargetPort:
		return normalizeSearchableText(joinDistinctNonEmpty(
			getRequestPort(entry),
			getResponsePort(entry),
		))
	case HistoryFilterTargetScheme:
		return normalizeSearchableText(getRequestScheme(entry))
	case HistoryFilterTargetPath:
		return normalizeSearchableText(getRequestPath(entry))
	default:
		panic("HistoryFilterTarget no configurado al Filtrar Peticiones")
	}
}

func normalizeHistoryFilterOperator(value HistoryFilterOperator) HistoryFilterOperator {
	if value == HistoryFilterOr {
		return HistoryFilterOr
	}
	return HistoryFilterAnd
}

func normalizeHistoryFilterTarget(value HistoryFilterTarget) HistoryFilterTarget {
	switch value {
	case HistoryFilterTargetAll,
		HistoryFilterTargetRequest,
		HistoryFilterTargetResponse,
		HistoryFilterTargetRequestHead,
		HistoryFilterTargetResponseHead,
		HistoryFilterTargetRequestBody,
		HistoryFilterTargetResponseBody,
		HistoryFilterTargetHead,
		HistoryFilterTargetBody,
		HistoryFilterTargetMethod,
		HistoryFilterTargetHost,
		HistoryFilterTargetPort,
		HistoryFilterTargetScheme,
		HistoryFilterTargetPath:
		return value
	default:
		panic("HistoryFilterTarget no configurado al Filtrar Peticiones")
	}
}

func normalizeHistoryFilterMatchMode(value HistoryFilterMatchMode) HistoryFilterMatchMode {
	switch value {
	case HistoryMatchContains,
		HistoryMatchNotContains,
		HistoryMatchEquals,
		HistoryMatchNotEquals,
		HistoryMatchStartsWith,
		HistoryMatchEndsWith:
		return value
	default:
		panic("HistoryFilterMatchMode no configurado al Filtrar Peticiones")
	}
}

func normalizeSearchableText(value string) string {
	return strings.ToLower(value)
}

func uniqueUint64s(values []uint64) []uint64 {
	if len(values) == 0 {
		return []uint64{}
	}

	seen := make(map[uint64]struct{}, len(values))
	unique := make([]uint64, 0, len(values))

	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}

	return unique
}

func joinNonEmpty(values ...string) string {
	// #8 Une strings ignorando cadenas vacías
	parts := make([]string, 0, len(values))
	totalLen := 0

	for _, value := range values {
		if value == "" {
			continue
		}
		parts = append(parts, value)
		totalLen += len(value)
	}

	if len(parts) == 0 {
		return ""
	}

	totalLen += len(parts) - 1
	var builder strings.Builder
	builder.Grow(totalLen)

	for index, part := range parts {
		if index > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(part)
	}

	return builder.String()
}

func joinDistinctNonEmpty(values ...string) string {
	// #8 Une strings ignorando cadenas vacías y cadenas duplicadas
	seen := make(map[string]struct{}, len(values))
	parts := make([]string, 0, len(values))
	totalLen := 0

	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		parts = append(parts, value)
		totalLen += len(value)
	}

	if len(parts) == 0 {
		return ""
	}

	totalLen += len(parts) - 1
	var builder strings.Builder
	builder.Grow(totalLen)

	for index, part := range parts {
		if index > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(part)
	}

	return builder.String()
}

func getRequestHead(entry *HTTPHistoryEntryDetail) string {
	if entry == nil || entry.Request == nil {
		return ""
	}
	return entry.Request.HeadBlockStr
}

func getRequestBody(entry *HTTPHistoryEntryDetail) string {
	if entry == nil || entry.Request == nil {
		return ""
	}
	return entry.Request.BodyStr
}

func getResponseHead(entry *HTTPHistoryEntryDetail) string {
	if entry == nil || entry.Response == nil {
		return ""
	}
	return entry.Response.HeadBlockStr
}

func getResponseBody(entry *HTTPHistoryEntryDetail) string {
	if entry == nil || entry.Response == nil {
		return ""
	}
	return entry.Response.BodyStr
}

func getRequestHost(entry *HTTPHistoryEntryDetail) string {
	if entry == nil || entry.Request == nil {
		return ""
	}
	return entry.Request.Host
}

func getResponseHost(entry *HTTPHistoryEntryDetail) string {
	if entry == nil || entry.Response == nil {
		return ""
	}
	return entry.Response.Host
}

func getRequestPort(entry *HTTPHistoryEntryDetail) string {
	if entry == nil || entry.Request == nil {
		return ""
	}
	return entry.Request.Port
}

func getResponsePort(entry *HTTPHistoryEntryDetail) string {
	if entry == nil || entry.Response == nil {
		return ""
	}
	return entry.Response.Port
}

func getRequestScheme(entry *HTTPHistoryEntryDetail) string {
	if entry == nil || entry.Request == nil {
		return ""
	}
	return entry.Request.Scheme
}

func getRequestPath(entry *HTTPHistoryEntryDetail) string {
	if entry == nil || entry.Request == nil {
		return ""
	}
	return entry.Request.Path
}

func getRequestMethod(entry *HTTPHistoryEntryDetail) string {
	if entry == nil || entry.Request == nil {
		return ""
	}
	return entry.Request.Method
}
