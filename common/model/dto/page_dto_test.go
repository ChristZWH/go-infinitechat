package dto

import "testing"

func TestNewPageResponse(t *testing.T) {
	tests := []struct {
		name         string
		total        int
		pageNum      int
		pageSize     int
		wantPageNum  int64
		wantPageSize int64
		wantPages    int64
		wantHasNext  bool
		wantHasPrev  bool
	}{
		{"首页", 100, 1, 10, 1, 10, 10, true, false},
		{"中间页", 100, 5, 10, 5, 10, 10, true, true},
		{"末页", 100, 10, 10, 10, 10, 10, false, true},
		{"页码为 0 兜底为 1", 100, 0, 10, 1, 10, 10, true, false},
		{"页码为负兜底为 1", 100, -5, 10, 1, 10, 10, true, false},
		{"pageSize 为 0 兜底为 10", 100, 1, 0, 1, 10, 10, true, false},
		{"空列表", 0, 1, 10, 1, 10, 0, false, false},
		{"总数不整除", 11, 2, 10, 2, 10, 2, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewPageResponse([]string{}, tt.total, tt.pageNum, tt.pageSize)

			if resp.PageNum != tt.wantPageNum {
				t.Errorf("PageNum = %d, 期望 %d", resp.PageNum, tt.wantPageNum)
			}
			if resp.PageSize != tt.wantPageSize {
				t.Errorf("PageSize = %d, 期望 %d", resp.PageSize, tt.wantPageSize)
			}
			if resp.Pages != tt.wantPages {
				t.Errorf("Pages = %d, 期望 %d", resp.Pages, tt.wantPages)
			}
			if resp.HasNext != tt.wantHasNext {
				t.Errorf("HasNext = %v, 期望 %v", resp.HasNext, tt.wantHasNext)
			}
			if resp.HasPrevious != tt.wantHasPrev {
				t.Errorf("HasPrevious = %v, 期望 %v", resp.HasPrevious, tt.wantHasPrev)
			}
			if resp.Total != int64(tt.total) {
				t.Errorf("Total = %d, 期望 %d", resp.Total, tt.total)
			}
		})
	}
}

func TestNewPageResponseNilList(t *testing.T) {
	resp := NewPageResponse[string](nil, 0, 1, 10)
	if resp.List == nil {
		t.Fatal("List 不应为 nil（JSON 序列化会变成 null，前端难以处理）")
	}
}

func TestNewPageResponseFromAll(t *testing.T) {
	all := []int{1, 2, 3, 4, 5, 6, 7}

	tests := []struct {
		name      string
		pageNum   int
		pageSize  int
		wantLen   int
		wantFirst int
	}{
		{"第 1 页", 1, 3, 3, 1},
		{"第 2 页", 2, 3, 3, 4},
		{"最后一页不满", 3, 3, 1, 7},
		{"页码超出范围返回空列表", 10, 3, 0, 0},
		{"pageSize 超上限被截断", 1, 1000, 7, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewPageResponseFromAll(all, tt.pageNum, tt.pageSize)
			if len(resp.List) != tt.wantLen {
				t.Fatalf("列表长度 = %d, 期望 %d", len(resp.List), tt.wantLen)
			}
			if tt.wantLen > 0 && resp.List[0] != tt.wantFirst {
				t.Errorf("首元素 = %d, 期望 %d", resp.List[0], tt.wantFirst)
			}
			if resp.PageSize > MaxPageSize {
				t.Errorf("PageSize = %d 超过上限 %d", resp.PageSize, MaxPageSize)
			}
		})
	}
}

func TestPageRequestNormalize(t *testing.T) {
	pr := &PageRequest{PageNum: 0, PageSize: 0}
	pr.Normalize()
	if pr.PageNum != 1 || pr.PageSize != 10 {
		t.Fatalf("非法参数归一化后 = (%d,%d), 期望 (1,10)", pr.PageNum, pr.PageSize)
	}

	pr = &PageRequest{PageNum: -3, PageSize: 100000}
	pr.Normalize()
	if pr.PageNum != 1 || pr.PageSize != MaxPageSize {
		t.Fatalf("越界参数归一化后 = (%d,%d), 期望 (1,%d)", pr.PageNum, pr.PageSize, MaxPageSize)
	}
}
