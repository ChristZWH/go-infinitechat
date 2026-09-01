package model

const MaxPageSize = 100

type PageRequest struct {
	PageNum  int64 `json:"pageNum" form:"pageNum"`
	PageSize int64 `json:"pageSize" form:"pageSize"`
}

type PageResponse[T any] struct {
	List        []T   `json:"list"`
	Total       int64 `json:"total"`
	PageNum     int64 `json:"pageNum"`
	PageSize    int64 `json:"pageSize"`
	Pages       int64 `json:"pages"`
	HasNext     bool  `json:"hasNext"`
	HasPrevious bool  `json:"hasPrevious"`
}

func NewPageResponseFromAll[T any](allData []T, pageNum, pageSize int) *PageResponse[T] {
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize >= MaxPageSize {
		pageSize = MaxPageSize
	}
	if len(allData) == 0 {
		return &PageResponse[T]{
			List:        []T{},
			Total:       0,
			PageNum:     int64(pageNum),
			PageSize:    int64(pageSize),
			Pages:       0,
			HasNext:     false,
			HasPrevious: pageNum > 1,
		}
	}

	total := int64(len(allData))
	pages := (total + int64(pageSize) - 1) / int64(pageSize)

	fromIndex := (pageNum - 1) * pageSize
	toIndex := min(fromIndex+pageSize, len(allData))

	var list []T
	if fromIndex < len(allData) {
		list = allData[fromIndex:toIndex]
	} else {
		list = []T{}
	}

	return &PageResponse[T]{
		List:        list,
		Total:       total,
		PageNum:     int64(pageNum),
		PageSize:    int64(pageSize),
		Pages:       pages,
		HasNext:     int64(pageNum) < pages,
		HasPrevious: pageNum > 1,
	}
}

func NewPageResponse[T any](list []T, total, pageNum, pageSize int) *PageResponse[T] {
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if list == nil {
		list = []T{}
	}
	pages := (int64(total) + int64(pageSize) - 1) / int64(pageSize)
	return &PageResponse[T]{
		List:        list,
		Total:       int64(total),
		PageNum:     int64(pageNum),
		PageSize:    int64(pageSize),
		Pages:       pages,
		HasNext:     int64(pageNum) < pages,
		HasPrevious: pageNum > 1,
	}
}

func (pr *PageRequest) Normalize() {
	if pr.PageNum <= 0 {
		pr.PageNum = 1
	}
	if pr.PageSize <= 0 {
		pr.PageSize = 10
	}
	if pr.PageSize >= MaxPageSize {
		pr.PageSize = MaxPageSize
	}
}
