package thread

import (
	"container/list"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/burugo/thing"
	"github.com/jieliapp/jieli/backend/model"
)

const (
	threadListScanLimit              = 300
	longThreadMessagesCacheThreshold = 200
	longThreadMessagesCacheMaxBytes  = 64 << 20
)

var threadFacetsCache = struct {
	sync.RWMutex
	byUserID map[int64]cachedThreadFacetsEntry
}{byUserID: make(map[int64]cachedThreadFacetsEntry)}

var ErrThreadNotFound = errors.New("thread not found")

type cachedThreadFacetsEntry struct {
	db                *sql.DB
	userUpdatedAtUnix int64
	usedAgentsJSON    string
	facets            Facets
}

type ListOptions struct {
	Search     string
	Provider   string
	Repo       string
	Label      string
	Visibility string
	Archived   *bool
	Sort       string
	Page       int
	PageSize   int
}

type ListResult struct {
	Threads  []Summary `json:"threads"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

type FindOptions struct {
	Query    string
	Provider string
	Limit    int
	Offset   int
}

type FindResult struct {
	Threads []FindThread `json:"threads"`
	HasMore bool         `json:"hasMore"`
}

type FindThread struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	CreatorUserID     string `json:"creatorUserID"`
	Created           int64  `json:"created"`
	UpdatedAt         string `json:"updatedAt"`
	MessageCount      int    `json:"messageCount"`
	MatchedSearchText string `json:"matchedSearchText,omitempty"`
}

type Summary struct {
	ID               int64     `json:"id"`
	Provider         string    `json:"provider"`
	ProviderThreadID string    `json:"provider_thread_id"`
	Title            string    `json:"title"`
	Visibility       string    `json:"visibility"`
	ShareSlug        string    `json:"share_slug"`
	Repo             string    `json:"repo"`
	RepoURL          string    `json:"repo_url"`
	Branch           string    `json:"branch"`
	SourceURL        string    `json:"source_url"`
	Preview          string    `json:"preview"`
	WorkingDirectory string    `json:"working_directory,omitempty"`
	MessageCount     int       `json:"message_count"`
	LinesAdded       int       `json:"lines_added"`
	LinesRemoved     int       `json:"lines_removed"`
	LinesChanged     int       `json:"lines_changed"`
	FilesChanged     int       `json:"files_changed"`
	Labels           []string  `json:"labels"`
	Archived         bool      `json:"archived"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Detail struct {
	ID               int64           `json:"id"`
	Provider         string          `json:"provider"`
	ProviderThreadID string          `json:"provider_thread_id"`
	Owner            *PublicOwner    `json:"owner,omitempty"`
	CreatorUserID    string          `json:"creator_user_id"`
	Title            string          `json:"title"`
	Visibility       string          `json:"visibility"`
	ShareSlug        string          `json:"share_slug"`
	Repo             string          `json:"repo"`
	RepoURL          string          `json:"repo_url"`
	Branch           string          `json:"branch"`
	WorkspaceRoot    string          `json:"workspace_root,omitempty"`
	WorkingDirectory string          `json:"working_directory,omitempty"`
	SourceURL        string          `json:"source_url"`
	Messages         json.RawMessage `json:"messages"`
	Mode             string          `json:"mode"`
	Model            string          `json:"model,omitempty"`
	ReasoningEffort  string          `json:"reasoning_effort"`
	SummaryStats     map[string]any  `json:"summary_stats,omitempty"`
	ContextPercent   *float64        `json:"context_percent,omitempty"`
	ContextWindow    *int            `json:"context_window_tokens,omitempty"`
	CostDisplay      string          `json:"cost_display,omitempty"`
	MessageCount     int             `json:"message_count"`
	LinesAdded       int             `json:"lines_added"`
	LinesRemoved     int             `json:"lines_removed"`
	LinesChanged     int             `json:"lines_changed"`
	FilesChanged     int             `json:"files_changed"`
	Labels           []string        `json:"labels"`
	Archived         bool            `json:"archived"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type PublicOwner struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
}

type envMetadata struct {
	Initial struct {
		Trees []struct {
			Repository struct {
				Ref string `json:"ref"`
			} `json:"repository"`
		} `json:"trees"`
		WorkspaceRoot    string `json:"workspaceRoot"`
		WorkingDirectory string `json:"workingDirectory"`
	} `json:"initial"`
}

type MetaUpdate struct {
	Title      string   `json:"title"`
	Visibility string   `json:"visibility"`
	Labels     []string `json:"labels"`
}

func ListThreads(user *model.User, options ListOptions) (ListResult, error) {
	if user == nil || user.ID == 0 {
		return ListResult{}, errors.New("user is required")
	}
	page := options.Page
	if page < 1 {
		page = 1
	}
	pageSize := options.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	where, args, err := threadListWhere(user.ID, options)
	if err != nil {
		return ListResult{}, err
	}
	query := model.ThreadDB.Where(where, args...)
	total, err := query.Count()
	if err != nil {
		return ListResult{}, err
	}
	offset := (page - 1) * pageSize
	threads, err := query.Order(threadListOrder(options.Sort)).Fetch(offset, pageSize)
	if err != nil {
		return ListResult{}, err
	}
	labelsByThreadID, err := labelsForThreads(threads)
	if err != nil {
		return ListResult{}, err
	}
	summaries := make([]Summary, 0, len(threads))
	for _, stored := range threads {
		summaries = append(summaries, summaryForThread(stored, labelsByThreadID[stored.ID], previewForStoredThread(stored)))
	}
	return ListResult{Threads: summaries, Total: int(total), Page: page, PageSize: pageSize}, nil
}

func threadListWhere(userID int64, options ListOptions) (string, []any, error) {
	clauses := []string{"user_id = ?"}
	args := []any{userID}
	if options.Provider != "" {
		clauses = append(clauses, "provider = ?")
		args = append(args, options.Provider)
	}
	if options.Repo != "" {
		clauses = append(clauses, "repo = ?")
		args = append(args, options.Repo)
	}
	if options.Visibility != "" {
		clauses = append(clauses, "visibility = ?")
		args = append(args, options.Visibility)
	}
	if options.Archived != nil {
		clauses = append(clauses, "archived = ?")
		args = append(args, *options.Archived)
	}
	if options.Search != "" {
		clauses = append(clauses, "search_text LIKE ?")
		args = append(args, "%"+strings.ToLower(options.Search)+"%")
	}
	if options.Label != "" {
		threadIDs, err := threadIDsForLabel(userID, options.Label)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, "id IN (?)")
		args = append(args, threadIDs)
	}
	return strings.Join(clauses, " AND "), args, nil
}

// threadIDsForLabel returns the IDs of the user's threads carrying the given
// label. Filtering the main list by this ID list (id IN (?)) keeps the thread
// list/count queries cache-friendly; a correlated subquery on thread_labels
// would defeat Thing's cache invalidation. An empty result is fine: id IN (?)
// with an empty slice expands to IN (NULL) and matches nothing.
func threadIDsForLabel(userID int64, label string) ([]int64, error) {
	rows, err := model.ThreadLabelDB.Where("user_id = ? AND label = ?", userID, label).All()
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ThreadID)
	}
	return ids, nil
}

func threadListOrder(sortBy string) string {
	if sortBy == "updated" {
		return "updated_at DESC, id DESC"
	}
	return "created_at DESC, id DESC"
}

func FindThreads(user *model.User, options FindOptions) (FindResult, error) {
	if user == nil || user.ID == 0 {
		return FindResult{}, errors.New("user is required")
	}
	limit := normalizeFindLimit(options.Limit)
	offset := options.Offset
	if offset < 0 {
		offset = 0
	}
	query := parseFindQuery(options.Query)
	threads, err := model.ThreadDB.Where("user_id = ?", user.ID).Fetch(0, threadListScanLimit)
	if err != nil {
		return FindResult{}, err
	}
	matched := make([]*model.Thread, 0, len(threads))
	for _, stored := range threads {
		if !matchesFindOptions(stored, options.Provider, query) {
			continue
		}
		matched = append(matched, stored)
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if threadUpdatedMillis(matched[i]) != threadUpdatedMillis(matched[j]) {
			return threadUpdatedMillis(matched[i]) > threadUpdatedMillis(matched[j])
		}
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].ID > matched[j].ID
	})
	start := offset
	if start > len(matched) {
		start = len(matched)
	}
	end := start + limit
	hasMore := end < len(matched)
	if end > len(matched) {
		end = len(matched)
	}
	results := make([]FindThread, 0, end-start)
	for _, stored := range matched[start:end] {
		results = append(results, findThreadForStored(stored, query))
	}
	return FindResult{Threads: results, HasMore: hasMore}, nil
}

const (
	defaultFindLimit = 20
	maxFindLimit     = 100
)

func normalizeFindLimit(limit int) int {
	if limit < 1 {
		return defaultFindLimit
	}
	if limit > maxFindLimit {
		return maxFindLimit
	}
	return limit
}

func sortThreadSummaries(threads []Summary, sortBy string) {
	sort.SliceStable(threads, func(i, j int) bool {
		if sortBy == "updated" {
			if !threads[i].UpdatedAt.Equal(threads[j].UpdatedAt) {
				return threads[i].UpdatedAt.After(threads[j].UpdatedAt)
			}
		}
		if !threads[i].CreatedAt.Equal(threads[j].CreatedAt) {
			return threads[i].CreatedAt.After(threads[j].CreatedAt)
		}
		return threads[i].ID > threads[j].ID
	})
}

func GetThreadDetail(user *model.User, threadID int64) (Detail, error) {
	if user == nil || user.ID == 0 {
		return Detail{}, errors.New("user is required")
	}
	if threadID == 0 {
		return Detail{}, errors.New("thread id is required")
	}
	stored, err := model.ThreadDB.ByID(threadID)
	if err != nil {
		return Detail{}, err
	}
	if stored.Deleted || stored.UserID != user.ID {
		return Detail{}, errors.New("thread not found")
	}
	labels, err := labelsForThread(stored.ID)
	if err != nil {
		return Detail{}, err
	}
	return detailForThread(stored, labels), nil
}

func GetThreadDetailByProviderID(user *model.User, providerThreadID string) (Detail, error) {
	if user == nil || user.ID == 0 {
		return Detail{}, errors.New("user is required")
	}
	if providerThreadID == "" {
		return Detail{}, errors.New("provider_thread_id is required")
	}
	threads, err := model.ThreadDB.Where("user_id = ? AND provider_thread_id = ?", user.ID, providerThreadID).Fetch(0, 1)
	if err != nil {
		return Detail{}, err
	}
	if len(threads) == 0 {
		return Detail{}, errors.New("thread not found")
	}
	labels, err := labelsForThread(threads[0].ID)
	if err != nil {
		return Detail{}, err
	}
	return detailForThread(threads[0], labels), nil
}

// Facets is the data used to populate the threads-list filter dropdowns.
type Facets struct {
	Providers []model.ProviderInfo `json:"providers"`
	Repos     []string             `json:"repos"`
	Labels    []string             `json:"labels"`
}

// ThreadFacets returns the user's used providers plus the distinct, sorted
// repos and labels for a user's threads. The user-derived sets are independent
// (not cross-filtered).
func ThreadFacets(user *model.User) (Facets, error) {
	if user == nil || user.ID == 0 {
		return Facets{}, errors.New("user is required")
	}
	latestUser, err := model.UserDB.ByID(user.ID)
	if err != nil {
		return Facets{}, err
	}
	if cached, ok := cachedThreadFacets(user.ID, latestUser); ok {
		return cached, nil
	}
	repos, err := distinctThreadRepos(user.ID)
	if err != nil {
		return Facets{}, err
	}
	labels, err := distinctUserLabels(user.ID)
	if err != nil {
		return Facets{}, err
	}
	facets := Facets{
		Providers: model.UsedProviderInfos(latestUser),
		Repos:     repos,
		Labels:    labels,
	}
	storeThreadFacets(user.ID, latestUser, facets)
	return cloneFacets(facets), nil
}

func cachedThreadFacets(userID int64, user *model.User) (Facets, bool) {
	threadFacetsCache.RLock()
	defer threadFacetsCache.RUnlock()
	entry, ok := threadFacetsCache.byUserID[userID]
	if !ok || user == nil || entry.db != model.ThreadDB.DB() || entry.userUpdatedAtUnix != user.UpdatedAt.UnixNano() || entry.usedAgentsJSON != user.UsedAgentsJSON {
		return Facets{}, false
	}
	return cloneFacets(entry.facets), true
}

func storeThreadFacets(userID int64, user *model.User, facets Facets) {
	threadFacetsCache.Lock()
	defer threadFacetsCache.Unlock()
	threadFacetsCache.byUserID[userID] = cachedThreadFacetsEntry{
		db:                model.ThreadDB.DB(),
		userUpdatedAtUnix: user.UpdatedAt.UnixNano(),
		usedAgentsJSON:    user.UsedAgentsJSON,
		facets:            cloneFacets(facets),
	}
}

func invalidateUserFacets(userID int64) {
	if userID == 0 {
		return
	}
	threadFacetsCache.Lock()
	defer threadFacetsCache.Unlock()
	delete(threadFacetsCache.byUserID, userID)
}

func cloneFacets(facets Facets) Facets {
	return Facets{
		Providers: append([]model.ProviderInfo(nil), facets.Providers...),
		Repos:     append([]string(nil), facets.Repos...),
		Labels:    append([]string(nil), facets.Labels...),
	}
}

func distinctThreadRepos(userID int64) ([]string, error) {
	return scanDistinctStrings(
		"SELECT DISTINCT repo FROM threads WHERE user_id = ? AND deleted = ? AND repo <> '' ORDER BY repo ASC",
		userID, false,
	)
}

func distinctUserLabels(userID int64) ([]string, error) {
	return scanDistinctStrings(
		"SELECT DISTINCT label FROM thread_labels WHERE user_id = ? AND deleted = ? AND label <> '' ORDER BY label ASC",
		userID, false,
	)
}

func scanDistinctStrings(query string, args ...any) ([]string, error) {
	rows, err := model.ThreadDB.DB().QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func GetSharedThread(shareSlug string) (Detail, error) {
	if shareSlug == "" {
		return Detail{}, errors.New("share slug is required")
	}
	threads, err := model.ThreadDB.Where("share_slug = ? AND visibility = ?", shareSlug, model.ThreadVisibilityUnlisted).Fetch(0, 1)
	if err != nil {
		return Detail{}, err
	}
	return publicDetailForSingleThread(threads)
}

func GetPublicThreadDetailByProviderID(providerThreadID string) (Detail, error) {
	if providerThreadID == "" {
		return Detail{}, errors.New("provider_thread_id is required")
	}
	threads, err := model.ThreadDB.Where("provider_thread_id = ? AND visibility = ?", providerThreadID, model.ThreadVisibilityUnlisted).Fetch(0, 2)
	if err != nil {
		return Detail{}, err
	}
	return publicDetailForSingleThread(threads)
}

func publicDetailForSingleThread(threads []*model.Thread) (Detail, error) {
	if len(threads) != 1 {
		return Detail{}, errors.New("thread not found")
	}
	if threads[0].Deleted {
		return Detail{}, errors.New("thread not found")
	}
	labels, err := labelsForThread(threads[0].ID)
	if err != nil {
		return Detail{}, err
	}
	return detailForThread(threads[0], labels), nil
}

func UpdateThreadMeta(user *model.User, threadID int64, update MetaUpdate) (*model.Thread, error) {
	if user == nil || user.ID == 0 {
		return nil, errors.New("user is required")
	}
	stored, err := model.ThreadDB.ByID(threadID)
	if err != nil {
		return nil, err
	}
	if stored.Deleted || stored.UserID != user.ID {
		return nil, errors.New("thread not found")
	}
	if update.Title != "" {
		stored.Title = update.Title
	}
	if update.Visibility != "" {
		if update.Visibility != model.ThreadVisibilityPrivate && update.Visibility != model.ThreadVisibilityUnlisted {
			return nil, errors.New("invalid visibility")
		}
		stored.Visibility = update.Visibility
		if update.Visibility == model.ThreadVisibilityUnlisted && stored.ShareSlug == "" {
			stored.ShareSlug = newShareSlug()
		}
	}
	if err := model.ThreadDB.Save(stored); err != nil {
		return nil, err
	}
	if update.Labels != nil {
		if _, err := replaceLabels(user.ID, stored.ID, update.Labels); err != nil {
			return nil, err
		}
	}
	return stored, nil
}

func SetThreadArchived(user *model.User, threadID int64, archived bool) (*model.Thread, error) {
	if user == nil || user.ID == 0 {
		return nil, errors.New("user is required")
	}
	stored, err := model.ThreadDB.ByID(threadID)
	if err != nil {
		return nil, err
	}
	if stored.Deleted || stored.UserID != user.ID {
		return nil, errors.New("thread not found")
	}
	stored.Archived = archived
	if err := model.ThreadDB.Save(stored); err != nil {
		return nil, err
	}
	return stored, nil
}

func SetProviderThreadArchived(user *model.User, provider string, providerThreadID string, archived bool) (*model.Thread, error) {
	if user == nil || user.ID == 0 {
		return nil, errors.New("user is required")
	}
	provider = strings.TrimSpace(provider)
	if !model.IsSupportedProvider(provider) {
		return nil, errors.New("invalid provider")
	}
	if strings.TrimSpace(providerThreadID) == "" {
		return nil, ErrThreadNotFound
	}
	stored, err := existingThread(user.ID, provider, providerThreadID)
	if err != nil {
		return nil, err
	}
	if stored == nil || stored.Deleted {
		return nil, ErrThreadNotFound
	}
	stored.Archived = archived
	if err := model.ThreadDB.Save(stored); err != nil {
		return nil, err
	}
	return stored, nil
}

func DeleteThread(user *model.User, threadID int64) error {
	if user == nil || user.ID == 0 {
		return errors.New("user is required")
	}
	stored, err := model.ThreadDB.ByID(threadID)
	if err != nil {
		return err
	}
	if stored.Deleted || stored.UserID != user.ID {
		return errors.New("thread not found")
	}
	return hardDeleteThread(stored)
}

func SoftDeleteThread(user *model.User, threadID int64) error {
	if user == nil || user.ID == 0 {
		return errors.New("user is required")
	}
	stored, err := model.ThreadDB.ByID(threadID)
	if err != nil {
		return err
	}
	if stored.Deleted || stored.UserID != user.ID {
		return errors.New("thread not found")
	}
	return model.ThreadDB.SoftDelete(stored)
}

func hardDeleteThread(stored *model.Thread) error {
	return thing.Transaction(context.Background(), func(tx *thing.TxSession) error {
		if err := subtractThreadFromDailyActivity(tx, stored); err != nil {
			return err
		}
		for _, statement := range []string{
			"DELETE FROM thread_messages WHERE thread_id = ?",
			"DELETE FROM thread_labels WHERE thread_id = ?",
			"DELETE FROM thread_attachments WHERE thread_id = ?",
		} {
			if _, err := tx.Exec(statement, stored.ID); err != nil {
				return err
			}
		}
		tx.Touch("thread_messages", "thread_labels", "thread_attachments")
		threads, err := thing.UseTx[*model.Thread](tx)
		if err != nil {
			return err
		}
		return threads.Delete(stored)
	})
}

// subtractThreadFromDailyActivity reclaims the deleted thread's lifetime
// counters from the user's daily_activity ledger. daily_activity currently does
// not store per-thread per-day deltas, so this is an aggregate reclaim: it walks
// the newest activity rows first and subtracts only from available positive
// counters. This keeps lifetime profile totals aligned after explicit user
// deletion, but it does not preserve the original historical date distribution.
// Auto-cleanup uses SoftDeleteThread and intentionally does not call this path.
func subtractThreadFromDailyActivity(tx *thing.TxSession, stored *model.Thread) error {
	if stored == nil || stored.UserID == 0 || stored.MessageCount == 0 {
		return nil
	}
	activities, err := thing.UseTx[*model.DailyActivity](tx)
	if err != nil {
		return err
	}
	activityRows, err := activities.Where("user_id = ?", stored.UserID).Order("date DESC").All()
	if err != nil {
		return err
	}
	stats := parseSummaryStats(json.RawMessage(stored.SummaryStatsJSON))
	remainingThreads := 1
	remainingMessages := stored.MessageCount
	remainingAdded := stored.LinesAdded
	remainingRemoved := stored.LinesRemoved
	remainingChanged := stats.Changed
	remainingFiles := stored.FilesChanged
	for _, row := range activityRows {
		nextThreads, usedThreads := subtractAvailable(row.ThreadCount, remainingThreads)
		nextMessages, usedMessages := subtractAvailable(row.MessageCount, remainingMessages)
		nextAdded, usedAdded := subtractAvailable(row.LinesAdded, remainingAdded)
		nextRemoved, usedRemoved := subtractAvailable(row.LinesRemoved, remainingRemoved)
		nextChanged, usedChanged := subtractAvailable(row.LinesChanged, remainingChanged)
		nextFiles, usedFiles := subtractAvailable(row.FilesChanged, remainingFiles)
		if usedThreads == 0 && usedMessages == 0 && usedAdded == 0 && usedRemoved == 0 && usedChanged == 0 && usedFiles == 0 {
			continue
		}
		row.ThreadCount = nextThreads
		row.MessageCount = nextMessages
		row.LinesAdded = nextAdded
		row.LinesRemoved = nextRemoved
		row.LinesChanged = nextChanged
		row.FilesChanged = nextFiles
		if err := activities.Save(row); err != nil {
			return err
		}
		remainingThreads -= usedThreads
		remainingMessages -= usedMessages
		remainingAdded -= usedAdded
		remainingRemoved -= usedRemoved
		remainingChanged -= usedChanged
		remainingFiles -= usedFiles
		if remainingThreads == 0 && remainingMessages == 0 && remainingAdded == 0 && remainingRemoved == 0 && remainingChanged == 0 && remainingFiles == 0 {
			break
		}
	}
	return nil
}

func subtractAvailable(current int, remaining int) (int, int) {
	if current <= 0 || remaining <= 0 {
		return current, 0
	}
	used := minInt(current, remaining)
	return current - used, used
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func labelsForThread(threadID int64) ([]string, error) {
	rows, err := model.ThreadLabelDB.Where("thread_id = ?", threadID).Fetch(0, 1000)
	if err != nil {
		return nil, err
	}
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, row.Label)
	}
	return labels, nil
}

func labelsForThreads(threads []*model.Thread) (map[int64][]string, error) {
	labelsByThreadID := make(map[int64][]string, len(threads))
	if len(threads) == 0 {
		return labelsByThreadID, nil
	}
	ids := make([]int64, 0, len(threads))
	for _, stored := range threads {
		labelsByThreadID[stored.ID] = nil
		ids = append(ids, stored.ID)
	}
	rows, err := model.ThreadLabelDB.Where("thread_id IN (?)", ids).All()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		labelsByThreadID[row.ThreadID] = append(labelsByThreadID[row.ThreadID], row.Label)
	}
	return labelsByThreadID, nil
}

type previewCandidate struct {
	position int
	text     string
}

func previewsForThreads(threads []*model.Thread) (map[int64]string, error) {
	previewsByThreadID := make(map[int64]string, len(threads))
	if len(threads) == 0 {
		return previewsByThreadID, nil
	}
	ids := make([]int64, 0, len(threads))
	for _, stored := range threads {
		previewsByThreadID[stored.ID] = preview(normalizePreviewText(stored.Title))
		ids = append(ids, stored.ID)
	}
	rows, err := model.ThreadMessageDB.Where("thread_id IN (?)", ids).All()
	if err != nil {
		return nil, err
	}
	candidatesByThreadID := make(map[int64][]previewCandidate, len(threads))
	for _, row := range rows {
		text := normalizePreviewText(textFromStoredMessage(row))
		if text == "" {
			continue
		}
		candidate := previewCandidate{position: row.Position, text: text}
		candidatesByThreadID[row.ThreadID] = append(candidatesByThreadID[row.ThreadID], candidate)
	}
	for _, stored := range threads {
		candidates := candidatesByThreadID[stored.ID]
		if len(candidates) == 0 {
			continue
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].position < candidates[j].position
		})
		parts := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			parts = append(parts, candidate.text)
		}
		previewsByThreadID[stored.ID] = preview(strings.Join(parts, " "))
	}
	return previewsByThreadID, nil
}

func textFromStoredMessage(message *model.ThreadMessage) string {
	if message == nil {
		return ""
	}
	rawJSON, err := message.RawJSON()
	if err != nil || rawJSON == "" {
		return ""
	}
	var decoded struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &decoded); err == nil && len(decoded.Content) != 0 {
		return textFromContent(decoded.Content)
	}
	var legacy struct {
		Content json.RawMessage `json:"Content"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &legacy); err == nil && len(legacy.Content) != 0 {
		return textFromContent(legacy.Content)
	}
	return ""
}

func normalizePreviewText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func previewFromStoredMessages(messages []storedMessage) string {
	parts := make([]string, 0, 2)
	for _, message := range messages {
		text := normalizePreviewText(textFromContent(message.Content))
		if text == "" {
			continue
		}
		parts = append(parts, text)
		if len(parts) == 2 {
			break
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return preview(strings.Join(parts, " "))
}

func previewForStoredThread(stored *model.Thread) string {
	if stored.Preview != "" {
		return preview(normalizePreviewText(stored.Preview))
	}
	return preview(normalizePreviewText(stored.Title))
}

func matchesOptions(stored *model.Thread, labels []string, options ListOptions) bool {
	if options.Provider != "" && stored.Provider != options.Provider {
		return false
	}
	if options.Repo != "" && stored.Repo != options.Repo {
		return false
	}
	if options.Visibility != "" && stored.Visibility != options.Visibility {
		return false
	}
	if options.Archived != nil && stored.Archived != *options.Archived {
		return false
	}
	if options.Label != "" && !hasLabel(labels, options.Label) {
		return false
	}
	if options.Search != "" && !strings.Contains(strings.ToLower(stored.SearchText), strings.ToLower(options.Search)) {
		return false
	}
	return true
}

type findQuery struct {
	Terms        []string
	Repo         string
	Ref          string
	Author       string
	AfterMillis  int64
	HasAfter     bool
	BeforeMillis int64
	HasBefore    bool
}

func parseFindQuery(raw string) findQuery {
	query := findQuery{}
	for _, token := range findTokens(strings.TrimSpace(raw)) {
		key, value, ok := strings.Cut(token, ":")
		if !ok || value == "" {
			query.Terms = append(query.Terms, token)
			continue
		}
		switch strings.ToLower(key) {
		case "repo":
			query.Repo = normalizeRepoFilter(value)
		case "ref":
			query.Ref = value
		case "author":
			query.Author = value
		case "file":
			query.Terms = append(query.Terms, value)
		case "after":
			if millis, ok := parseFindTimeMillis(value); ok {
				query.AfterMillis = millis
				query.HasAfter = true
			}
		case "before":
			if millis, ok := parseFindTimeMillis(value); ok {
				query.BeforeMillis = millis
				query.HasBefore = true
			}
		default:
			query.Terms = append(query.Terms, token)
		}
	}
	return query
}

func findTokens(input string) []string {
	tokens := []string{}
	var current strings.Builder
	inQuote := false
	for _, char := range input {
		switch {
		case char == '"':
			inQuote = !inQuote
		case !inQuote && char <= ' ':
			if current.Len() != 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}
	if current.Len() != 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func parseFindTimeMillis(value string) (int64, bool) {
	if len(value) > 1 {
		unit := value[len(value)-1]
		amount, err := strconv.Atoi(value[:len(value)-1])
		if err == nil && amount >= 0 {
			switch unit {
			case 'd':
				return time.Now().Add(-time.Duration(amount) * 24 * time.Hour).UnixMilli(), true
			case 'w':
				return time.Now().Add(-time.Duration(amount) * 7 * 24 * time.Hour).UnixMilli(), true
			}
		}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UnixMilli(), true
		}
	}
	return 0, false
}

func matchesFindOptions(stored *model.Thread, provider string, query findQuery) bool {
	provider = strings.TrimSpace(provider)
	if provider != "" && stored.Provider != provider {
		return false
	}
	if query.Repo != "" && normalizeRepoFilter(stored.Repo) != query.Repo {
		return false
	}
	if query.Ref != "" && branchForThread(stored) != branchFromRef(query.Ref) {
		return false
	}
	if query.Author != "" && strings.ToLower(query.Author) != "me" && !strings.Contains(strings.ToLower(stored.CreatorUserID), strings.ToLower(query.Author)) {
		return false
	}
	updatedMillis := threadUpdatedMillis(stored)
	if query.HasAfter && updatedMillis < query.AfterMillis {
		return false
	}
	if query.HasBefore && updatedMillis >= query.BeforeMillis {
		return false
	}
	searchable := strings.ToLower(strings.Join([]string{stored.ProviderThreadID, stored.Title, stored.Repo, branchForThread(stored), stored.SourceURL, stored.SearchText}, "\n"))
	for _, term := range query.Terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term != "" && !strings.Contains(searchable, term) {
			return false
		}
	}
	return true
}

func normalizeRepoFilter(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "https://github.com/")
	value = strings.TrimPrefix(value, "http://github.com/")
	value = strings.TrimSuffix(value, ".git")
	return strings.Trim(value, "/")
}

func findThreadForStored(stored *model.Thread, query findQuery) FindThread {
	return FindThread{
		ID:                stored.ProviderThreadID,
		Title:             stored.Title,
		CreatorUserID:     stored.CreatorUserID,
		Created:           providerCreatedMillis(stored),
		UpdatedAt:         providerUpdatedAt(stored),
		MessageCount:      stored.MessageCount,
		MatchedSearchText: matchedFindSnippet(stored.SearchText, query.Terms),
	}
}

func matchedFindSnippet(searchText string, terms []string) string {
	searchText = strings.TrimSpace(searchText)
	if searchText == "" || len(terms) == 0 {
		return ""
	}
	lower := strings.ToLower(searchText)
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		index := strings.Index(lower, term)
		if index < 0 {
			continue
		}
		start := index - 60
		if start < 0 {
			start = 0
		}
		end := index + len(term) + 80
		if end > len(searchText) {
			end = len(searchText)
		}
		return strings.TrimSpace(searchText[start:end])
	}
	return preview(searchText)
}

func threadUpdatedMillis(stored *model.Thread) int64 {
	if stored.UserLastInteractedAtMS != 0 {
		return stored.UserLastInteractedAtMS
	}
	if stored.ProviderUpdatedAt != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, stored.ProviderUpdatedAt); err == nil {
			return parsed.UnixMilli()
		}
	}
	return stored.UpdatedAt.UnixMilli()
}

func hasLabel(labels []string, target string) bool {
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
}

func summaryForThread(stored *model.Thread, labels []string, previewText string) Summary {
	stats := changeStatsForStoredThread(stored, nil)
	return Summary{
		ID:               stored.ID,
		Provider:         stored.Provider,
		ProviderThreadID: stored.ProviderThreadID,
		Title:            stored.Title,
		Visibility:       stored.Visibility,
		ShareSlug:        stored.ShareSlug,
		Repo:             stored.Repo,
		RepoURL:          stored.RepoURL,
		Branch:           branchForThread(stored),
		SourceURL:        stored.SourceURL,
		Preview:          previewText,
		WorkingDirectory: workspaceDirForThread(stored),
		MessageCount:     stored.MessageCount,
		LinesAdded:       stats.Added,
		LinesRemoved:     stats.Deleted,
		LinesChanged:     stats.Changed,
		FilesChanged:     stats.FilesChanged,
		Labels:           labels,
		Archived:         stored.Archived,
		CreatedAt:        stored.CreatedAt,
		UpdatedAt:        stored.UpdatedAt,
	}
}

func preview(searchText string) string {
	text := strings.TrimSpace(searchText)
	return truncateRunes(text, 160)
}

func truncateRunes(text string, limit int) string {
	if limit < 0 {
		limit = 0
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

func newShareSlug() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("thread-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", buffer)
}

func detailForThread(stored *model.Thread, labels []string) Detail {
	payload, _ := threadMessagePayloadForThread(stored)
	stats := payload.Stats
	if isZeroChangeStats(stats) {
		stats = changeStatsForStoredThread(stored, nil)
	}
	return Detail{
		ID:               stored.ID,
		Provider:         stored.Provider,
		ProviderThreadID: stored.ProviderThreadID,
		Owner:            publicOwnerForUserID(stored.UserID),
		CreatorUserID:    stored.CreatorUserID,
		Title:            stored.Title,
		Visibility:       stored.Visibility,
		ShareSlug:        stored.ShareSlug,
		Repo:             stored.Repo,
		RepoURL:          stored.RepoURL,
		Branch:           branchForThread(stored),
		WorkspaceRoot:    workspaceRootForThread(stored),
		WorkingDirectory: workingDirectoryForThread(stored),
		SourceURL:        stored.SourceURL,
		Messages:         payload.Messages,
		Mode:             stored.AgentMode,
		Model:            modelForThread(stored),
		ReasoningEffort:  stored.ReasoningEffort,
		SummaryStats:     summaryStatsForThread(stored),
		ContextPercent:   payload.ContextPercent,
		ContextWindow:    payload.ContextWindow,
		MessageCount:     stored.MessageCount,
		LinesAdded:       stats.Added,
		LinesRemoved:     stats.Deleted,
		LinesChanged:     stats.Changed,
		FilesChanged:     stats.FilesChanged,
		Labels:           labels,
		Archived:         stored.Archived,
		CreatedAt:        stored.CreatedAt,
		UpdatedAt:        stored.UpdatedAt,
	}
}

func changeStatsForStoredThread(stored *model.Thread, messages []*model.ThreadMessage) threadChangeStats {
	stats := parseSummaryStats(json.RawMessage(stored.SummaryStatsJSON))
	if stats.Added == 0 {
		stats.Added = stored.LinesAdded
	}
	if stats.Deleted == 0 {
		stats.Deleted = stored.LinesRemoved
	}
	if stats.FilesChanged == 0 {
		stats.FilesChanged = stored.FilesChanged
	}
	if !hasLineChangeStats(stats) && messages != nil {
		messageStats := changeStatsForStoredMessages(messages)
		if !isZeroChangeStats(messageStats) {
			return messageStats
		}
	}
	if !isZeroChangeStats(stats) {
		return stats
	}
	if messages == nil {
		return stats
	}
	return changeStatsForStoredMessages(messages)
}

func changeStatsForStoredMessages(messages []*model.ThreadMessage) threadChangeStats {
	stats := threadChangeStats{}
	files := make(map[string]struct{})
	for _, message := range messages {
		var raw struct {
			Content json.RawMessage `json:"content"`
		}
		rawJSON, err := message.RawJSON()
		if err != nil {
			continue
		}
		if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
			continue
		}
		for _, edit := range editFileInputs(raw.Content) {
			files[edit.Path] = struct{}{}
			added, deleted, changed := lineDiffStats(edit.OldStr, edit.NewStr)
			stats.Added += added
			stats.Deleted += deleted
			stats.Changed += changed
		}
	}
	stats.FilesChanged = len(files)
	return stats
}

func publicOwnerForUserID(userID int64) *PublicOwner {
	user, err := model.UserDB.ByID(userID)
	if err != nil || user == nil {
		return nil
	}
	return &PublicOwner{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
	}
}

func summaryStatsForThread(stored *model.Thread) map[string]any {
	if stored.SummaryStatsJSON == "" {
		return nil
	}
	var stats map[string]any
	if err := json.Unmarshal([]byte(stored.SummaryStatsJSON), &stats); err != nil {
		return nil
	}
	return stats
}

func modelForThread(stored *model.Thread) string {
	if stored.MetaJSON == "" {
		return ""
	}
	var meta struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal([]byte(stored.MetaJSON), &meta); err != nil {
		return ""
	}
	return meta.Model
}

func branchForThread(stored *model.Thread) string {
	if stored.Branch != "" {
		return stored.Branch
	}
	env := envForThread(stored)
	for _, tree := range env.Initial.Trees {
		if branch := branchFromRef(tree.Repository.Ref); branch != "" {
			return branch
		}
	}
	return ""
}

func workspaceRootForThread(stored *model.Thread) string {
	env := envForThread(stored)
	return env.Initial.WorkspaceRoot
}

func workingDirectoryForThread(stored *model.Thread) string {
	env := envForThread(stored)
	return env.Initial.WorkingDirectory
}

// workspaceDirForThread returns the working directory, falling back to the
// workspace root, mirroring the sidebar's folder display logic.
func workspaceDirForThread(stored *model.Thread) string {
	env := envForThread(stored)
	if env.Initial.WorkingDirectory != "" {
		return env.Initial.WorkingDirectory
	}
	return env.Initial.WorkspaceRoot
}

func envForThread(stored *model.Thread) envMetadata {
	var env envMetadata
	if stored.EnvJSON == "" {
		return env
	}
	_ = json.Unmarshal([]byte(stored.EnvJSON), &env)
	return env
}

func orderedThreadMessages(threadID int64) ([]*model.ThreadMessage, error) {
	return model.ThreadMessageDB.Where("thread_id = ?", threadID).Order("position ASC").All()
}

type threadMessagePayload struct {
	Messages       json.RawMessage
	ContextPercent *float64
	ContextWindow  *int
	Stats          threadChangeStats
}

type cachedThreadMessagePayload struct {
	db             *sql.DB
	messageCount   int
	threadV        int64
	updatedAtUnix  int64
	messages       json.RawMessage
	contextPercent *float64
	contextWindow  *int
	stats          threadChangeStats
	sizeBytes      int64
	lruElement     *list.Element
}

type threadMessagePayloadCache struct {
	sync.Mutex
	maxBytes   int64
	usedBytes  int64
	byThreadID map[int64]*cachedThreadMessagePayload
	lru        list.List
}

func newThreadMessagePayloadCache(maxBytes int64) *threadMessagePayloadCache {
	return &threadMessagePayloadCache{
		maxBytes:   maxBytes,
		byThreadID: make(map[int64]*cachedThreadMessagePayload),
	}
}

var threadMessagesCache = newThreadMessagePayloadCache(longThreadMessagesCacheMaxBytes)

func messagesForThread(stored *model.Thread) (json.RawMessage, error) {
	payload, err := threadMessagePayloadForThread(stored)
	if err != nil {
		return nil, err
	}
	return payload.Messages, nil
}

func threadMessagePayloadForThread(stored *model.Thread) (threadMessagePayload, error) {
	if stored == nil || stored.ID == 0 {
		return threadMessagePayload{}, errors.New("thread is required")
	}
	if stored.MessageCount > longThreadMessagesCacheThreshold {
		if cached, ok := cachedThreadMessagePayloadFor(stored); ok {
			return cached, nil
		}
	}
	messages, err := orderedThreadMessages(stored.ID)
	if err != nil {
		return threadMessagePayload{}, err
	}
	raw, err := rawMessages(messages)
	if err != nil {
		return threadMessagePayload{}, err
	}
	contextPercent, contextWindow := contextUsageForMessages(messages)
	payload := threadMessagePayload{
		Messages:       raw,
		ContextPercent: contextPercent,
		ContextWindow:  contextWindow,
		Stats:          changeStatsForStoredThread(stored, messages),
	}
	if stored.MessageCount > longThreadMessagesCacheThreshold {
		storeThreadMessagePayload(stored, payload)
	}
	return payload, nil
}

func cachedThreadMessagePayloadFor(stored *model.Thread) (threadMessagePayload, bool) {
	return threadMessagesCache.load(model.ThreadDB.DB(), stored)
}

func (cache *threadMessagePayloadCache) load(db *sql.DB, stored *model.Thread) (threadMessagePayload, bool) {
	cache.Lock()
	cached, ok := cache.byThreadID[stored.ID]
	if !ok {
		cache.Unlock()
		return threadMessagePayload{}, false
	}
	if cached.db != db || cached.messageCount != stored.MessageCount || cached.threadV != stored.ThreadV || cached.updatedAtUnix != stored.UpdatedAt.UnixNano() {
		cache.removeLocked(stored.ID)
		cache.Unlock()
		return threadMessagePayload{}, false
	}
	cache.lru.MoveToFront(cached.lruElement)
	cache.Unlock()

	return threadMessagePayload{
		Messages:       append(json.RawMessage(nil), cached.messages...),
		ContextPercent: cloneFloat64Ptr(cached.contextPercent),
		ContextWindow:  cloneIntPtr(cached.contextWindow),
		Stats:          cached.stats,
	}, true
}

func storeThreadMessagePayload(stored *model.Thread, payload threadMessagePayload) {
	threadMessagesCache.store(model.ThreadDB.DB(), stored, payload)
}

func (cache *threadMessagePayloadCache) store(db *sql.DB, stored *model.Thread, payload threadMessagePayload) {
	sizeBytes := int64(len(payload.Messages))
	cache.Lock()
	defer cache.Unlock()
	cache.removeLocked(stored.ID)
	if cache.maxBytes <= 0 || sizeBytes > cache.maxBytes {
		return
	}
	entry := &cachedThreadMessagePayload{
		db:             db,
		messageCount:   stored.MessageCount,
		threadV:        stored.ThreadV,
		updatedAtUnix:  stored.UpdatedAt.UnixNano(),
		messages:       append(json.RawMessage(nil), payload.Messages...),
		contextPercent: cloneFloat64Ptr(payload.ContextPercent),
		contextWindow:  cloneIntPtr(payload.ContextWindow),
		stats:          payload.Stats,
		sizeBytes:      sizeBytes,
	}
	entry.lruElement = cache.lru.PushFront(stored.ID)
	cache.byThreadID[stored.ID] = entry
	cache.usedBytes += sizeBytes
	for cache.usedBytes > cache.maxBytes {
		oldest := cache.lru.Back()
		if oldest == nil {
			break
		}
		cache.removeLocked(oldest.Value.(int64))
	}
}

func invalidateThreadMessagesCache(threadID int64) {
	if threadID == 0 {
		return
	}
	threadMessagesCache.remove(threadID)
}

func (cache *threadMessagePayloadCache) remove(threadID int64) {
	cache.Lock()
	defer cache.Unlock()
	cache.removeLocked(threadID)
}

func (cache *threadMessagePayloadCache) removeLocked(threadID int64) {
	cached, ok := cache.byThreadID[threadID]
	if !ok {
		return
	}
	delete(cache.byThreadID, threadID)
	cache.lru.Remove(cached.lruElement)
	cache.usedBytes -= cached.sizeBytes
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func rawMessages(messages []*model.ThreadMessage) (json.RawMessage, error) {
	items := make([]json.RawMessage, 0, len(messages))
	for _, message := range messages {
		rawJSON, err := message.RawJSON()
		if err != nil {
			return nil, err
		}
		items = append(items, json.RawMessage(rawJSON))
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func contextUsageForMessages(messages []*model.ThreadMessage) (*float64, *int) {
	var latest *struct {
		TotalInputTokens int `json:"totalInputTokens"`
		MaxInputTokens   int `json:"maxInputTokens"`
	}
	for _, message := range messages {
		var raw struct {
			Usage struct {
				TotalInputTokens int `json:"totalInputTokens"`
				MaxInputTokens   int `json:"maxInputTokens"`
			} `json:"usage"`
		}
		rawJSON, err := message.RawJSON()
		if err != nil {
			continue
		}
		if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
			continue
		}
		if raw.Usage.MaxInputTokens <= 0 {
			continue
		}
		usage := raw.Usage
		latest = &usage
	}
	if latest == nil {
		return nil, nil
	}
	window := latest.MaxInputTokens
	percent := math.Round((float64(latest.TotalInputTokens)/float64(latest.MaxInputTokens))*1000) / 10
	return &percent, &window
}

func providerUpdatedAt(stored *model.Thread) string {
	if stored.ProviderUpdatedAt != "" {
		return stored.ProviderUpdatedAt
	}
	return stored.UpdatedAt.UTC().Format(time.RFC3339Nano)
}

func providerCreatedMillis(stored *model.Thread) int64 {
	if stored.ProviderCreatedMillis != 0 {
		return stored.ProviderCreatedMillis
	}
	return stored.CreatedAt.UnixMilli()
}
