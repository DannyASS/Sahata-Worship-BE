package handler

import (
	"net/http/httptest"
	"testing"
)

func TestPaginationRequest(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		page     int
		pageSize int
		search   string
	}{
		{name: "defaults invalid values", query: "?page=0&pageSize=0", page: 1, pageSize: 5},
		{name: "keeps valid values", query: "?page=3&pageSize=4&search=%20Grace%20", page: 3, pageSize: 4, search: "Grace"},
		{name: "caps oversized page", query: "?page=2&pageSize=100", page: 2, pageSize: 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/v1/songs"+test.query, nil)
			got := paginationRequest(r)
			if got.Page != test.page || got.PageSize != test.pageSize || got.Search != test.search {
				t.Fatalf("paginationRequest() = %+v, want page=%d pageSize=%d search=%q", got, test.page, test.pageSize, test.search)
			}
		})
	}
}

func TestHasPagination(t *testing.T) {
	withPage := httptest.NewRequest("GET", "/api/v1/cues?page=1", nil)
	withoutPage := httptest.NewRequest("GET", "/api/v1/cues", nil)
	if !hasPagination(withPage) {
		t.Fatal("hasPagination() = false, want true")
	}
	if hasPagination(withoutPage) {
		t.Fatal("hasPagination() = true, want false")
	}
}
