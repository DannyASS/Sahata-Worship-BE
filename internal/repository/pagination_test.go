package repository

import (
	"reflect"
	"testing"

	"sahata-worship-be/internal/domain"
)

func TestPage(t *testing.T) {
	got := page([]int{6, 7, 8, 9, 10}, 12, domain.PageRequest{Page: 2, PageSize: 5})
	if got.Total != 12 || got.Page != 2 || got.PageSize != 5 || got.TotalPages != 3 {
		t.Fatalf("page() metadata = %+v, want total=12 page=2 pageSize=5 totalPages=3", got)
	}
}

func TestSongFilter(t *testing.T) {
	where, args := songFilter("  Kasih  ")
	wantArgs := []any{"%Kasih%", "%Kasih%"}
	if where == "" || !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("songFilter() = %q, %#v; want LIKE filter and %#v", where, args, wantArgs)
	}
}

func TestSongPageFilter(t *testing.T) {
	where, args := songPageFilter(domain.PageRequest{
		Search:     "  Kasih  ",
		ExcludeIDs: []int64{3, 8},
	})
	wantWhere := ` WHERE (title LIKE ? OR artist LIKE ?) AND id NOT IN (?,?)`
	wantArgs := []any{"%Kasih%", "%Kasih%", int64(3), int64(8)}
	if where != wantWhere || !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("songPageFilter() = %q, %#v; want %q and %#v", where, args, wantWhere, wantArgs)
	}
}
