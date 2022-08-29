package api

import (
	"fmt"
	"strconv"
)

type PaginationResp struct {
	Ok     bool        `json:"ok"`
	Reason string      `json:"reason,omitempty"`
	Total  *uint       `json:"total,omitempty"`
	Size   uint        `json:"size,omitempty"`
	Page   uint        `json:"page,omitempty"`
	Data   interface{} `json:"data,omitempty" comment:"muster be a pointer of slice gorm.Model"` // save pagination list
}

type PaginationReq struct {
	Size uint
	Page uint
}

func NewPaginationReq(size, page string) PaginationReq {
	size_, _ := strconv.ParseUint(size, 10, 8)
	page_, _ := strconv.ParseUint(page, 10, 8)
	ret := PaginationReq{
		Size: uint(size_),
		Page: uint(page_),
	}
	if ret.Page <= 0 {
		ret.Page = 1
	}
	if ret.Size <= 0 {
		ret.Size = 10
	}
	return ret
}

func NewPaginationRespOk(req PaginationReq, total uint, data interface{}) PaginationResp {
	return PaginationResp{
		Ok:    true,
		Total: &total,
		Size:  req.Size,
		Page:  req.Page,
		Data:  data,
	}
}

func NewPaginationRespSingle(ok bool, data interface{}) PaginationResp {
	var total uint = 1
	return PaginationResp{
		Ok:    ok,
		Total: &total,
		Size:  1,
		Page:  1,
		Data:  data,
	}
}

func NewPaginationRespError(reason string) PaginationResp {
	return PaginationResp{
		Ok:     false,
		Reason: reason,
	}
}

func PaginationToSql(orderBy string, req PaginationReq) string {
	if req.Size > 0 {
		limit := req.Size
		offset := (req.Page - 1) * req.Size
		return fmt.Sprintf(" order by %s limit %d offset %d", orderBy, limit, offset)
	} else {
		return ""
	}
}
