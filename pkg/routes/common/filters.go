package common

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

// FilterFunc is a function that takes http request and GORM DB adds a query according to the request
type FilterFunc func(r *http.Request, tx *gorm.DB) *gorm.DB

// Filter is the struct that defines an API Filter
type Filter struct {
	QueryParam string
	DBField    string
}

// ContainFilterHandler handles sub string values
func ContainFilterHandler(filter *Filter) FilterFunc {
	sqlQuery := fmt.Sprintf("%s LIKE ?", filter.DBField)
	return FilterFunc(func(r *http.Request, tx *gorm.DB) *gorm.DB {
		if val := r.URL.Query().Get(filter.QueryParam); val != "" {
			tx = tx.Where(sqlQuery, "%"+val+"%")
		}
		return tx
	})
}

// OneOfFilterHandler handles multiple values filters
func OneOfFilterHandler(filter *Filter) FilterFunc {
	sqlQuery := fmt.Sprintf("%s IN ?", filter.DBField)
	return FilterFunc(func(r *http.Request, tx *gorm.DB) *gorm.DB {
		if vals, ok := r.URL.Query()[filter.QueryParam]; ok {
			tx = tx.Where(sqlQuery, vals)
		}
		return tx
	})
}

// LayoutISO represent the date layout in the API query
const LayoutISO = "2006-01-02"

// CreatedAtFilterHandler handles the "created_at" filter
func CreatedAtFilterHandler(filter *Filter) FilterFunc {
	return FilterFunc(func(r *http.Request, tx *gorm.DB) *gorm.DB {
		if val := r.URL.Query().Get(filter.QueryParam); val != "" {
			currentDay, err := time.Parse(LayoutISO, val)
			if err != nil {
				return tx
			}
			nextDay := currentDay.Add(time.Hour * 24)
			tx = tx.Where("%s BETWEEN ? AND ?", filter.DBField, currentDay.Format(LayoutISO), nextDay.Format(LayoutISO))
		}
		return tx
	})
}

// SortFilterHandler handles sorting
func SortFilterHandler(sortTable, defaultSortKey, defaultOrder string) FilterFunc {
	return FilterFunc(func(r *http.Request, tx *gorm.DB) *gorm.DB {
		sortBy := defaultSortKey
		sortOrder := defaultOrder
		if val := r.URL.Query().Get("sort_by"); val != "" {
			if strings.HasPrefix(val, "-") {
				sortOrder = "DESC"
				sortBy = val[1:]
			} else {
				sortOrder = "ASC"
				sortBy = val
			}
		}
		return tx.Order(fmt.Sprintf("%s.%s %s", sortTable, sortBy, sortOrder))
	})
}

// ComposeFilters composes all the filters into one function
func ComposeFilters(fs ...FilterFunc) FilterFunc {
	return func(r *http.Request, tx *gorm.DB) *gorm.DB {
		for _, f := range fs {
			tx = f(r, tx)
		}
		return tx
	}
}
