package note

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Cortalo/digitalgarden-backend/internal/domain/note"
	userservice "github.com/Cortalo/digitalgarden-backend/internal/service/user"
)

// mockRepository implements Repository.
type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) GetNoteBySlug(ctx context.Context, slug string) (note.Note, error) {
	args := m.Called(ctx, slug)
	n, _ := args.Get(0).(note.Note)
	return n, args.Error(1)
}

func (m *mockRepository) ListNotes(ctx context.Context, limit int32) ([]note.Note, error) {
	args := m.Called(ctx, limit)
	res, _ := args.Get(0).([]note.Note)
	return res, args.Error(1)
}

func (m *mockRepository) CreateNote(ctx context.Context, n note.Note) (note.Note, error) {
	args := m.Called(ctx, n)
	created, _ := args.Get(0).(note.Note)
	return created, args.Error(1)
}

// mockAuthorFinder implements AuthorFinder.
type mockAuthorFinder struct {
	mock.Mock
}

func (m *mockAuthorFinder) Get(ctx context.Context, id int64) (userservice.User, error) {
	args := m.Called(ctx, id)
	u, _ := args.Get(0).(userservice.User)
	return u, args.Error(1)
}

// mockSearchIndex implements SearchIndex.
type mockSearchIndex struct {
	mock.Mock
}

func (m *mockSearchIndex) IndexNote(ctx context.Context, n note.Note) error {
	args := m.Called(ctx, n)
	return args.Error(0)
}

func (m *mockSearchIndex) Search(ctx context.Context, keyword string, limit int32) ([]note.SearchHit, error) {
	args := m.Called(ctx, keyword, limit)
	res, _ := args.Get(0).([]note.SearchHit)
	return res, args.Error(1)
}

func newTestService(repo *mockRepository, authors *mockAuthorFinder, search *mockSearchIndex) *Service {
	return NewService(repo, authors, search)
}

const testMarkdown = "# A Title\n\nSome content.\n"

func TestPublish_IndexesOnSuccess(t *testing.T) {
	repo := new(mockRepository)
	authors := new(mockAuthorFinder)
	search := new(mockSearchIndex)
	svc := newTestService(repo, authors, search)

	authors.On("Get", mock.Anything, int64(7)).Return(userservice.User{ID: 7, Name: "Long"}, nil)
	created := note.Note{ID: 42, Title: "A Title", Slug: "a-title", AuthorUserID: 7, AuthorName: "Long"}
	repo.On("CreateNote", mock.Anything, mock.Anything).Return(created, nil)
	search.On("IndexNote", mock.Anything, created).Return(nil)

	got, err := svc.Publish(context.Background(), 7, "A Title", testMarkdown, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, created, got)
	search.AssertCalled(t, "IndexNote", mock.Anything, created)
}

// TestPublish_ReturnsNoteWhenIndexingFails is the core best-effort
// guarantee: Postgres is already the source of truth by the time
// IndexNote runs, so a search-index hiccup must never turn a successful
// publish into a failed one.
func TestPublish_ReturnsNoteWhenIndexingFails(t *testing.T) {
	repo := new(mockRepository)
	authors := new(mockAuthorFinder)
	search := new(mockSearchIndex)
	svc := newTestService(repo, authors, search)

	authors.On("Get", mock.Anything, int64(7)).Return(userservice.User{ID: 7, Name: "Long"}, nil)
	created := note.Note{ID: 42, Title: "A Title", Slug: "a-title", AuthorUserID: 7, AuthorName: "Long"}
	repo.On("CreateNote", mock.Anything, mock.Anything).Return(created, nil)
	search.On("IndexNote", mock.Anything, created).Return(errors.New("cluster unreachable"))

	got, err := svc.Publish(context.Background(), 7, "A Title", testMarkdown, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, created, got)
}

func TestSearch_DelegatesToSearchIndex(t *testing.T) {
	search := new(mockSearchIndex)
	svc := newTestService(new(mockRepository), new(mockAuthorFinder), search)

	want := []note.SearchHit{{Note: note.Note{ID: 1, Title: "Widget"}, Snippets: []string{"a <em>widget</em>"}}}
	search.On("Search", mock.Anything, "widget", int32(10)).Return(want, nil)

	got, err := svc.Search(context.Background(), "widget", 10)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestSearch_NonPositiveLimitUsesDefault(t *testing.T) {
	search := new(mockSearchIndex)
	svc := newTestService(new(mockRepository), new(mockAuthorFinder), search)

	search.On("Search", mock.Anything, "widget", int32(defaultSearchLimit)).Return([]note.SearchHit{}, nil)

	_, err := svc.Search(context.Background(), "widget", 0)

	require.NoError(t, err)
	search.AssertCalled(t, "Search", mock.Anything, "widget", int32(defaultSearchLimit))
}

func TestSearch_SearchIndexErrorPropagates(t *testing.T) {
	search := new(mockSearchIndex)
	svc := newTestService(new(mockRepository), new(mockAuthorFinder), search)

	searchErr := errors.New("cluster unreachable")
	search.On("Search", mock.Anything, "widget", int32(10)).Return(nil, searchErr)

	_, err := svc.Search(context.Background(), "widget", 10)

	assert.ErrorIs(t, err, searchErr)
}
